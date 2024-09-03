package build

import batchv1 "k8s.io/api/batch/v1"

type buildahConstructor struct{}

func (c *buildahConstructor) ConstructJobs() ([]batchv1.Job, error) {
	return nil, nil
}
