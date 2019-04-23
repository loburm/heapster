// Copyright 2015 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package gcl

import (
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	gce_util "github.com/Stackdriver/heapster/common/gce"
	"github.com/Stackdriver/heapster/events/core"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	gcl "google.golang.org/api/logging/v2"
	"k8s.io/klog"
)

const (
	monitoredResourceType = "global"
	logName               = "kubernetes.io/events"
	loggingSeverity       = "NOTICE"
)

type gclSink struct {
	project    string
	gclService *gcl.Service
}

func (sink *gclSink) ExportEvents(eventBatch *core.EventBatch) {
	if len(eventBatch.Events) == 0 {
		klog.V(4).Info("Not events to export")
		return
	}
	klog.V(4).Info("Exporting events")
	entries := make([]*gcl.LogEntry, len(eventBatch.Events))
	for i, event := range eventBatch.Events {
		evtJson, err := json.Marshal(event)
		if err != nil {
			klog.Errorf("Skipping exporting event due to error while marshaling event %v as JSON: %v", event, err)
			continue
		}
		entries[i] = &gcl.LogEntry{
			LogName:     fmt.Sprintf("projects/%s/logs/%s", sink.project, url.QueryEscape(logName)),
			Timestamp:   event.LastTimestamp.Time.UTC().Format(time.RFC3339),
			Severity:    loggingSeverity,
			Resource:    &gcl.MonitoredResource{Type: monitoredResourceType},
			InsertId:    string(event.UID),
			JsonPayload: evtJson,
		}
	}
	req := &gcl.WriteLogEntriesRequest{Entries: entries}
	if _, err := sink.gclService.Entries.Write(req).Do(); err != nil {
		klog.Errorf("Error while exporting events to GCL: %v", err)
	} else {
		klog.V(4).Infof("Successfully exported %d events", len(entries))
	}
}

func (sink *gclSink) Name() string {
	return "GCL Sink"
}

func (sink *gclSink) Stop() {
	// nothing needs to be done.
}

func CreateGCLSink(uri *url.URL) (core.EventSink, error) {
	client, err := google.DefaultClient(oauth2.NoContext, gcl.LoggingWriteScope)
	if err != nil {
		return nil, fmt.Errorf("error creating oauth2 client: %v", err)
	}

	// Create Google Cloud Logging service.
	gclService, err := gcl.New(client)
	if err != nil {
		return nil, fmt.Errorf("error creating GCL service: %v", err)
	}

	// Detect project ID
	projectId, err := gce_util.GetProjectId()
	if err != nil {
		return nil, fmt.Errorf("error getting GCP project ID: %v", err)
	}

	sink := &gclSink{project: projectId, gclService: gclService}
	klog.Info("created GCL sink")
	return sink, nil
}
