package utils

import (
	corev1 "k8s.io/api/core/v1"
)

type environmentVariablesSet struct {
	items    []corev1.EnvVar
	newItems []*corev1.EnvVar
	itemsMap map[string]*corev1.EnvVar
}

//NewEnvironmentVariablesSet Create new EnvironmentVariablesSet
func NewEnvironmentVariablesSet() *environmentVariablesSet {
	return (&environmentVariablesSet{}).Init(make([]corev1.EnvVar, 0))
}

//Init Init set with environment variables list
func (set *environmentVariablesSet) Init(environmentVariables []corev1.EnvVar) *environmentVariablesSet {
	set.items = environmentVariables
	set.newItems = make([]*corev1.EnvVar, 0)
	set.itemsMap = getMap(environmentVariables)
	return set
}

//Add Add an environment variable to the set
func (set *environmentVariablesSet) Add(name string, value string) *environmentVariablesSet {
	if _, ok := set.itemsMap[name]; ok {
		set.itemsMap[name].Value = value
		return set
	}
	set.newItems = append(set.newItems, &corev1.EnvVar{
		Name:  name,
		Value: value,
	})
	set.itemsMap[name] = set.newItems[len(set.newItems)-1]
	return set
}

//Items Get environment variables list from the set, ordered as they were added
func (set *environmentVariablesSet) Items() []corev1.EnvVar {
	var newItems []corev1.EnvVar
	for _, envVar := range set.newItems {
		newItems = append(newItems, *envVar)
	}
	return append(set.items, newItems...)
}

//Get Get an environment variable by name
func (set *environmentVariablesSet) Get(name string) (corev1.EnvVar, bool) {
	if envVar, ok := set.itemsMap[name]; ok {
		return *envVar, true
	}
	return corev1.EnvVar{}, false
}

func getMap(environmentVariables []corev1.EnvVar) map[string]*corev1.EnvVar {
	environmentVariablesMap := make(map[string]*corev1.EnvVar, len(environmentVariables))
	for i := 0; i < len(environmentVariables); i++ {
		environmentVariablesMap[environmentVariables[i].Name] = &(environmentVariables[i])
	}
	return environmentVariablesMap
}
