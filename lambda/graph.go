// Copyright 2022 Aspiro AB
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package lambda

import (
	"bytes"
	"fmt"
	"time"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/tidal-open-source/cw-alert-router/cw"
	"github.com/tidal-open-source/cw-alert-router/graph"
)

// GenerateMetricsGraphAndLink takes the cloudwatch event and generates a graph (stored on S3) - it will then
// return a url based on the configured endpoint (eg: cloudfront or s3 web hosting)
func GenerateMetricsGraphAndLink(detail *cw.EventDetails, cfg *Config) (string, error) {
	res, err := detail.GetMetricDataRequestForHrs()
	if err != nil {
		return "", err
	}
	imgBuf, err := graph.CreateFromCWMetricDataResult(res)
	if err != nil {
		return "", err
	}
	t := time.Now()
	id := uuid.New()
	objectKey := fmt.Sprintf("/%s/%d/%d/%d/%s.png", cfg.ImageBucketPrefix, t.Year(), t.Month(), t.Day(), id.String())
	bucket := cfg.ImageBucket

	log.Infof("writing graph to Bucket(%s) Key(%s)", bucket, objectKey)

	s3client := cfg.S3Client()
	err = s3client.WriteBytes(bucket, objectKey, bytes.NewReader(imgBuf.Bytes()))

	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s%s", cfg.ImageHost, objectKey), nil
}
