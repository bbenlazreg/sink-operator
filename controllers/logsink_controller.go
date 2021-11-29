/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	logv1alpha1 "wdm3.caas.vpod1.carrefour.com/cbbenlazreg/sink-operator/api/v1alpha1"

	"cloud.google.com/go/logging/logadmin"
)

const (
	projectName = "myproject"
)

// LogSinkReconciler reconciles a LogSink object
type LogSinkReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=log.carrefour.com,resources=logsinks,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=log.carrefour.com,resources=logsinks/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=log.carrefour.com,resources=logsinks/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the LogSink object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.10.0/pkg/reconcile
func (r *LogSinkReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	//_ = log.FromContext(ctx)
	log := ctrllog.FromContext(ctx)

	// your logic here
	logsink := &logv1alpha1.LogSink{}
	err := r.Get(ctx, req.NamespacedName, logsink)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			log.Info("LogSink resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, "Failed to get LogSink")
		return ctrl.Result{}, err
	}
	// Check if the logsink already exists in the project, if not create a new one
	//ctx := context.Background()
	client, err := logadmin.NewClient(ctx, projectName)
	if err != nil {
		log.Error(err, "Failed to init gcloud client")
		return ctrl.Result{}, err
	}
	// Use client to manage logs, metrics and sinks.
	// Close the client when finished.

	sink, err := client.Sink(ctx, logsink.Name)
	notFound := true
	if err != nil && notFound {
		log.Info("Sink not found")
		log.Info("Creating a new Sink")
		_, err := createSink(client, logsink.Name, logsink.Namespace, logsink.Spec.LogBucketURI)
		if err != nil {
			log.Error(err, "Failed to create sink")
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil

	} else if err != nil {
		log.Error(err, "Failed to get sink")
		return ctrl.Result{}, err
	}
	logBucketURI := logsink.Spec.LogBucketURI
	if sink.Destination != logBucketURI {

		_, err := updateSink(client, logsink.Name, logsink.Namespace, logBucketURI)
		if err != nil {
			log.Error(err, "Failed to update sink")
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Update status
	logsink.Status.SinkName = sink.ID
	logsink.Status.Destination = sink.Destination
	logsink.Status.Filter = sink.Filter
	logsink.Status.WriterIdentity = sink.WriterIdentity

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *LogSinkReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&logv1alpha1.LogSink{}).
		Complete(r)
}

//sink helpers

func createSink(client *logadmin.Client, name, namespace, logBucket string) (*logadmin.Sink, error) {
	// [START logging_create_sink]
	ctx := context.Background()
	sink, err := client.CreateSink(ctx, &logadmin.Sink{
		ID:          name,
		Destination: logBucket,
		Filter:      "resource.labels.namespace_name=\"" + namespace + "\"",
	})
	// [END logging_create_sink]
	return sink, err
}

func updateSink(client *logadmin.Client, name, namespace, logBucket string) (*logadmin.Sink, error) {
	// [START logging_update_sink]
	ctx := context.Background()
	sink, err := client.UpdateSink(ctx, &logadmin.Sink{
		ID:          name,
		Destination: logBucket,
		Filter:      "resource.labels.namespace_name=\"" + namespace + "\"",
	})
	// [END logging_update_sink]
	return sink, err
}

func deleteSink(client *logadmin.Client) error {
	// [START logging_delete_sink]
	ctx := context.Background()
	if err := client.DeleteSink(ctx, "severe-errors-to-gcs"); err != nil {
		return err
	}
	// [END logging_delete_sink]
	return nil
}
