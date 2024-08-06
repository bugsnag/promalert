package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/globalsign/mgo/bson"
	"github.com/pkg/errors"

	"github.com/bugsnag/bugsnag-go/v2"
)

func UploadFile(bucket, region string, plot io.WriterTo) (string, error) {
	s := session.Must(session.NewSession(&aws.Config{Region: aws.String(region)}))
	_, err := s.Config.Credentials.Get()
	if err != nil {
		return "", errors.Wrap(err, "failed to get AWS credentials")
	}

	f, err := os.CreateTemp("", "promplot-*.png")
	if err != nil {
		return "", errors.Wrap(err, "failed to create tmp file")
	}
	defer func() {
		err = f.Close()
		if err != nil {
			err = errors.Wrap(err, "failed to close tmp file")
			_ = bugsnag.Notify(err)
			panic(err)
		}
		err := os.Remove(f.Name())
		if err != nil {
			err = errors.Wrap(err, "failed to delete tmp file")
			_ = bugsnag.Notify(err)
			panic(err)
		}
	}()
	_, err = plot.WriteTo(f)
	if err != nil {
		return "", errors.Wrap(err, "failed to write plot to file")
	}

	// get the file size and read
	// the file content into a buffer
	fileInfo, _ := f.Stat()
	size := fileInfo.Size()
	buffer := make([]byte, size)
	_, err = f.Seek(0, io.SeekStart)
	_, err = f.Read(buffer)

	// create a unique file name for the file
	tempFileName := "pictures/" + bson.NewObjectId().Hex() + "_" + strconv.FormatInt(time.Now().Unix(), 10) + ".png"

	_, err = s3.New(s).PutObject(&s3.PutObjectInput{
		Bucket:        aws.String(bucket),
		Key:           aws.String(tempFileName),
		ACL:           aws.String("public-read"),
		Body:          bytes.NewReader(buffer),
		ContentLength: aws.Int64(int64(size)),
		ContentType:   aws.String(http.DetectContentType(buffer)),
	})
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("https://s3.amazonaws.com/%s/%s", bucket, tempFileName), err
}
