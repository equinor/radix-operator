package utils

import (
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"sort"
	"testing"
)

func Test_EmptySet(t *testing.T) {
	t.Run("Empty set", func(t *testing.T) {
		t.Parallel()
		set := NewEnvironmentVariablesSet()
		assert.Equal(t, 0, len(set.Items()))
	})
}

func Test_AddValuesToEmptySet(t *testing.T) {
	t.Run("Add one value", func(t *testing.T) {
		t.Parallel()
		set := NewEnvironmentVariablesSet().Add("name1", "value1")
		items := set.Items()
		assert.Equal(t, 1, len(items))
		assert.Equal(t, "name1", items[0].Name)
		assert.Equal(t, "value1", items[0].Value)
	})
	t.Run("Add multiple values", func(t *testing.T) {
		t.Parallel()
		set := NewEnvironmentVariablesSet().
			Add("name1", "value1").
			Add("name2", "value2").
			Add("name3", "value3")
		items := set.Items()
		assert.Equal(t, 3, len(items))
		assert.Equal(t, "name1", items[0].Name)
		assert.Equal(t, "value1", items[0].Value)
		assert.Equal(t, "name2", items[1].Name)
		assert.Equal(t, "value2", items[1].Value)
		assert.Equal(t, "name3", items[2].Name)
		assert.Equal(t, "value3", items[2].Value)
	})
}

func Test_AddValuesToNonEmptySet(t *testing.T) {
	t.Run("Add new value", func(t *testing.T) {
		t.Parallel()
		set := NewEnvironmentVariablesSet().Init([]corev1.EnvVar{
			{Name: "name1", Value: "value1"},
		})
		set.Add("name2", "value2")
		items := set.Items()
		sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
		assert.Equal(t, 2, len(items))
		assert.Equal(t, "name1", items[0].Name)
		assert.Equal(t, "value1", items[0].Value)
		assert.Equal(t, "name2", items[1].Name)
		assert.Equal(t, "value2", items[1].Value)
	})
	t.Run("Add multiple new values", func(t *testing.T) {
		t.Parallel()
		set := NewEnvironmentVariablesSet().Init([]corev1.EnvVar{
			{Name: "name1", Value: "value1"},
		})
		set.Add("name2", "value2").
			Add("name3", "value3").
			Add("name4", "value4")
		items := set.Items()
		assert.Equal(t, 4, len(items))
		assert.Equal(t, "name1", items[0].Name)
		assert.Equal(t, "value1", items[0].Value)
		assert.Equal(t, "name2", items[1].Name)
		assert.Equal(t, "value2", items[1].Value)
		assert.Equal(t, "name3", items[2].Name)
		assert.Equal(t, "value3", items[2].Value)
		assert.Equal(t, "name4", items[3].Name)
		assert.Equal(t, "value4", items[3].Value)
	})
	t.Run("Add var with existing name of non-empty set", func(t *testing.T) {
		t.Parallel()
		set := NewEnvironmentVariablesSet().Init([]corev1.EnvVar{
			{Name: "name2", Value: "value2"},
		})
		set.Add("name1", "value1").
			Add("name2", "value22").
			Add("name3", "value3")
		items := set.Items()
		assert.Equal(t, 3, len(items))
		assert.Equal(t, "name2", items[0].Name)
		assert.Equal(t, "value22", items[0].Value)
		assert.Equal(t, "name1", items[1].Name)
		assert.Equal(t, "value1", items[1].Value)
		assert.Equal(t, "name3", items[2].Name)
		assert.Equal(t, "value3", items[2].Value)
	})
}

func Test_GetItems(t *testing.T) {
	t.Run("Get items returns items with same order as they were added", func(t *testing.T) {
		t.Parallel()
		set := NewEnvironmentVariablesSet().Init([]corev1.EnvVar{
			{Name: "name3", Value: "value3"},
		})
		set.Add("name1", "value1").
			Add("name2", "value2")
		items := set.Items()
		assert.Equal(t, 3, len(items))
		assert.Equal(t, "name3", items[0].Name)
		assert.Equal(t, "value3", items[0].Value)
		assert.Equal(t, "name1", items[1].Name)
		assert.Equal(t, "value1", items[1].Value)
		assert.Equal(t, "name2", items[2].Name)
		assert.Equal(t, "value2", items[2].Value)
	})
}

func Test_GetValue(t *testing.T) {
	t.Run("Get existing value", func(t *testing.T) {
		t.Parallel()
		set := NewEnvironmentVariablesSet().
			Add("name1", "value1").
			Add("name2", "value2")
		envVar, found := set.Get("name2")
		assert.True(t, found)
		assert.Equal(t, "name2", envVar.Name)
		assert.Equal(t, "value2", envVar.Value)
	})
	t.Run("Get non existing value", func(t *testing.T) {
		t.Parallel()
		set := NewEnvironmentVariablesSet().
			Add("name1", "value1").
			Add("name2", "value2")
		_, found := set.Get("name3")
		assert.False(t, found)
	})
}
