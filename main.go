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

package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/tidal-music/cw-alert-router/v2/lambda"
)

func main() {
	h, err := lambda.New(context.Background(), lambda.ConfigFromEnv())
	if err != nil {
		slog.Error("failed initializing lambda", "error", err)
		os.Exit(1)
	}
	h.Start()
}
