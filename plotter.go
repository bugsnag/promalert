package main

import (
	"fmt"
	"image/color"
	"io"
	"regexp"
	"strconv"
	"time"

	"github.com/pkg/errors"
	"github.com/prometheus/prometheus/promql"

	"github.com/prometheus/common/model"
	"gonum.org/v1/plot"
	"gonum.org/v1/plot/palette/brewer"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"
	"gonum.org/v1/plot/vg/draw"

	"github.com/bugsnag/bugsnag-go/v2"
	"github.com/bugsnag/microkit/clog"
	"github.com/spf13/viper"
)

// Only show important part of metric name
var labelText = regexp.MustCompile("{(.*)}")

func GetPlotExpr(alertFormula string) []PlotExpr {
	expr, _ := promql.ParseExpr(alertFormula)
	if parenExpr, ok := expr.(*promql.ParenExpr); ok {
		expr = parenExpr.Expr
		clog.Infof("Removing redundant brackets: %v", expr.String())
	}

	if binaryExpr, ok := expr.(*promql.BinaryExpr); ok {
		var alertOperator string

		switch binaryExpr.Op {
		case promql.ItemLAND:
			clog.Warn("Logical condition, drawing sides separately")
			return append(GetPlotExpr(binaryExpr.LHS.String()), GetPlotExpr(binaryExpr.RHS.String())...)
		case promql.ItemLTE, promql.ItemLSS:
			alertOperator = "<"
		case promql.ItemGTE, promql.ItemGTR:
			alertOperator = ">"
		default:
			clog.Infof("Unexpected operator: %v", binaryExpr.Op.String())
			alertOperator = ">"
		}

		alertLevel, _ := strconv.ParseFloat(binaryExpr.RHS.String(), 64)
		return []PlotExpr{PlotExpr{
			Formula:  binaryExpr.LHS.String(),
			Operator: alertOperator,
			Level:    alertLevel,
		}}
	} else {
		clog.Infof("Non binary expression: %v", alertFormula)
		return nil
	}
}

func Plot(expr PlotExpr, queryTime time.Time, duration, resolution time.Duration, prometheusUrl string, alert Alert) (io.WriterTo, error) {
	clog.Infof("Querying Prometheus %s", expr.Formula)
	metrics, err := Metrics(
		prometheusUrl,
		expr.Formula,
		queryTime,
		duration,
		resolution,
	)
	if err != nil {
		_ = bugsnag.Notify(errors.Wrap(err, "error querying Prometheus"), nil,
			bugsnag.MetaData{
				"Expression": {
					"PrometheusUrl":      prometheusUrl,
					"ExpressionFormula":  expr.Formula,
					"ExpressionOperator": expr.Operator,
					"QueryTime":          queryTime.String(),
				},
				"Alert": {
					"GeneratorURL": alert.GeneratorURL,
					"Channel":      alert.Channel,
					"MessageTS":    alert.MessageTS,
				},
			})
		return nil, err
	}

	var selectedMetrics model.Matrix
	var found bool
	for _, metric := range metrics {
		clog.Infof("Metric fetched: %v", metric.Metric)
		found = false
		for label, value := range metric.Metric {
			if originValue, ok := alert.Labels[string(label)]; ok {
				if originValue == string(value) {
					found = true
				} else {
					found = false
					break
				}
			}
		}

		if found {
			clog.Infof("Best match found: %v", metric.Metric)
			selectedMetrics = model.Matrix{metric}
			break
		}
	}

	if !found {
		clog.Infof("Best match not found, use entire dataset. Labels to search: %v", alert.Labels)
		selectedMetrics = metrics
	}

	clog.Infof("Creating plot: %s", alert.Annotations["summary"])
	plottedMetric, err := PlotMetric(selectedMetrics, expr.Level, expr.Operator)
	if err != nil {
		_ = bugsnag.Notify(errors.Wrap(err, "error creating plot"), nil,
			bugsnag.MetaData{
				"Expression": {
					"PrometheusUrl":      prometheusUrl,
					"ExpressionFormula":  expr.Formula,
					"ExpressionOperator": expr.Operator,
					"QueryTime":          queryTime.String(),
				},
				"Alert": {
					"GeneratorURL": alert.GeneratorURL,
					"Channel":      alert.Channel,
					"MessageTS":    alert.MessageTS,
				},
			})
		return nil, err
	}

	return plottedMetric, nil
}

func PlotMetric(metrics model.Matrix, level float64, direction string) (io.WriterTo, error) {
	viper.SetDefault("graph_scale", 1.0)
	var graphScale = viper.GetFloat64("graph_scale")

	p, err := plot.New()
	if err != nil {
		return nil, errors.Wrap(err, "failed to create new plot")
	}

	textFont, err := vg.MakeFont("Helvetica", vg.Length(2.5*graphScale)*vg.Millimeter)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load font")
	}

	evalTextFont, err := vg.MakeFont("Helvetica", vg.Length(3*graphScale)*vg.Millimeter)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load font")
	}

	evalTextStyle := draw.TextStyle{
		Color:  color.NRGBA{A: 150},
		Font:   evalTextFont,
		XAlign: draw.XRight,
		YAlign: draw.YBottom,
	}

	//p.Y.Min = 0
	p.X.Tick.Marker = plot.TimeTicks{Format: "15:04:05"}
	p.X.Tick.Label.Font = textFont
	p.Y.Tick.Label.Font = textFont
	p.Legend.Font = textFont
	p.Legend.Top = true
	p.Legend.YOffs = vg.Length(15*graphScale) * vg.Millimeter

	// Color palette for drawing lines
	paletteSize := 8
	palette, err := brewer.GetPalette(brewer.TypeAny, "Dark2", paletteSize)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get color palette")
	}
	colors := palette.Colors()

	var lastEvalValue float64

	for s, sample := range metrics {
		data := make(plotter.XYs, 0)
		for _, v := range sample.Values {
			fs := v.Value.String()
			if fs == "NaN" {
				_, err := drawLine(data, colors, s, paletteSize, p, metrics, sample)
				if err != nil {
					return nil, err
				}

				data = make(plotter.XYs, 0)
				continue
			}

			f, err := strconv.ParseFloat(fs, 64)
			if err != nil {
				return nil, errors.Wrap(err, "sample value not float: "+v.Value.String())
			}
			data = append(data, plotter.XY{X: float64(v.Timestamp.Unix()), Y: f})
			lastEvalValue = f
		}

		_, err := drawLine(data, colors, s, paletteSize, p, metrics, sample)
		if err != nil {
			return nil, err
		}
	}

	var polygonPoints plotter.XYs

	if direction == "<" {
		polygonPoints = plotter.XYs{{X: p.X.Min, Y: level}, {X: p.X.Max, Y: level}, {X: p.X.Max, Y: p.Y.Min}, {X: p.X.Min, Y: p.Y.Min}}
	} else {
		polygonPoints = plotter.XYs{{X: p.X.Min, Y: level}, {X: p.X.Max, Y: level}, {X: p.X.Max, Y: p.Y.Max}, {X: p.X.Min, Y: p.Y.Max}}
	}

	poly, err := plotter.NewPolygon(polygonPoints)
	if err != nil {
		return nil, err
	}
	poly.Color = color.NRGBA{R: 255, A: 40}
	poly.LineStyle.Color = color.NRGBA{R: 0, A: 0}
	p.Add(poly)
	p.Add(plotter.NewGrid())

	// Draw plot in canvas with margin
	margin := vg.Length(3*graphScale) * vg.Millimeter
	width := vg.Length(12*graphScale) * vg.Centimeter
	height := vg.Length(6*graphScale) * vg.Centimeter
	c, err := draw.NewFormattedCanvas(width, height, "png")
	if err != nil {
		return nil, errors.Wrap(err, "failed to create canvas")
	}

	croppedCanvas := draw.Crop(draw.New(c), margin, -margin, margin, -margin)
	p.Draw(croppedCanvas)

	// Draw last evaluated value
	evalText := fmt.Sprintf("latest evaluation: %.2f", lastEvalValue)

	plotterCanvas := p.DataCanvas(croppedCanvas)

	trX, trY := p.Transforms(&plotterCanvas)
	evalRectangle := evalTextStyle.Rectangle(evalText)

	points := []vg.Point{
		{X: trX(p.X.Max) + evalRectangle.Min.X - 8*vg.Millimeter, Y: trY(lastEvalValue) + evalRectangle.Min.Y - vg.Millimeter},
		{X: trX(p.X.Max) + evalRectangle.Min.X - 8*vg.Millimeter, Y: trY(lastEvalValue) + evalRectangle.Max.Y + vg.Millimeter},
		{X: trX(p.X.Max) + evalRectangle.Max.X - 6*vg.Millimeter, Y: trY(lastEvalValue) + evalRectangle.Max.Y + vg.Millimeter},
		{X: trX(p.X.Max) + evalRectangle.Max.X - 6*vg.Millimeter, Y: trY(lastEvalValue) + evalRectangle.Min.Y - vg.Millimeter},
	}
	plotterCanvas.FillPolygon(color.NRGBA{R: 255, G: 255, B: 255, A: 90}, points)
	plotterCanvas.FillText(evalTextStyle, vg.Point{X: trX(p.X.Max) - 6*vg.Millimeter, Y: trY(lastEvalValue)}, evalText)

	return c, nil
}

func drawLine(data plotter.XYs, colors []color.Color, s int, paletteSize int, p *plot.Plot, metrics model.Matrix, sample *model.SampleStream) (*plotter.Line, error) {
	var l *plotter.Line
	var err error
	if len(data) > 0 {
		l, err = plotter.NewLine(data)
		if err != nil {
			return &plotter.Line{}, errors.Wrap(err, "failed to create line")
		}

		l.LineStyle.Width = vg.Points(1)
		l.LineStyle.Color = colors[s%paletteSize]

		p.Add(l)
		if len(metrics) > 1 {
			m := labelText.FindStringSubmatch(sample.Metric.String())
			if m != nil {
				p.Legend.Add(m[1], l)
			}
		}
	}

	return l, nil
}
