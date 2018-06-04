package brigade

import (
	log "github.com/Sirupsen/logrus"
	"github.com/prometheus/client_golang/prometheus"
	radix_v1 "github.com/statoil/radix-operator/pkg/apis/radix/v1"
	"k8s.io/client-go/kubernetes"
)

var projectCounter = prometheus.NewCounter(prometheus.CounterOpts{
	Name: "project_created",
	Help: "Number of projects created by the Radix Operator",
})

type BrigadeGateway struct {
	clientset kubernetes.Interface
}

func init() {
	prometheus.MustRegister(projectCounter)
}

func (b *BrigadeGateway) EnsureProject(app *radix_v1.RadixApplication) {
	log.Infof("Creating/Updating application %s", app.ObjectMeta.Name)
	projectCounter.Inc()
}

func (b *BrigadeGateway) DeleteProject(key string) {
	log.Infof("Removing project %s", key)
}
