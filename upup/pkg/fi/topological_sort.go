package fi

import (
	"crypto/x509/pkix"
	"fmt"
	"github.com/golang/glog"
	"k8s.io/kops/upup/pkg/fi/utils"
	"reflect"
)

type HasDependencies interface {
	GetDependencies(tasks map[string]Task) []Task
}

// FindTaskDependencies returns a map from each task's key to the discovered list of dependencies
func FindTaskDependencies(tasks map[string]Task) map[string][]string {
	taskToId := make(map[interface{}]string)
	for k, t := range tasks {
		taskToId[t] = k
	}

	edges := make(map[string][]string)

	for k, t := range tasks {
		task := t.(Task)

		var dependencies []Task
		if hd, ok := task.(HasDependencies); ok {
			dependencies = hd.GetDependencies(tasks)
		} else {
			dependencies = reflectForDependencies(tasks, task)
		}

		var dependencyKeys []string
		for _, dep := range dependencies {
			dependencyKey, found := taskToId[dep]
			if !found {
				glog.Fatalf("dependency not found: %v", dep)
			}
			dependencyKeys = append(dependencyKeys, dependencyKey)
		}

		edges[k] = dependencyKeys
	}

	glog.V(4).Infof("Dependencies:")
	for k, v := range edges {
		glog.V(4).Infof("\t%s:\t%v", k, v)
	}

	return edges
}

func reflectForDependencies(tasks map[string]Task, task Task) []Task {
	v := reflect.ValueOf(task).Elem()
	return getDependencies(tasks, v)
}

func getDependencies(tasks map[string]Task, v reflect.Value) []Task {
	var dependencies []Task

	err := utils.ReflectRecursive(v, func(path string, f *reflect.StructField, v reflect.Value) error {
		if utils.IsPrimitiveValue(v) {
			return nil
		}

		switch v.Kind() {
		case reflect.String:
			return nil

		case reflect.Interface, reflect.Ptr, reflect.Slice, reflect.Map:
			// The recursive walk will descend into this; we can ignore here
			return nil

		case reflect.Struct:
			if path == "" {
				// Ignore self - we are a struct, but not our own dependency!
				return nil
			}

			// TODO: Can we / should we use a type-switch statement
			intf := v.Addr().Interface()
			if hd, ok := intf.(HasDependencies); ok {
				deps := hd.GetDependencies(tasks)
				dependencies = append(dependencies, deps...)
			} else if dep, ok := intf.(Task); ok {
				dependencies = append(dependencies, dep)
			} else if _, ok := intf.(Resource); ok {
				// Ignore: not a dependency (?)
			} else if _, ok := intf.(*ResourceHolder); ok {
				// Ignore: not a dependency (?)
			} else if _, ok := intf.(*pkix.Name); ok {
				// Ignore: not a dependency
			} else {
				return fmt.Errorf("Unhandled type for %q: %T", path, v.Interface())
			}
			return utils.SkipReflection

		default:
			glog.Infof("Unhandled kind for %q: %T", path, v.Interface())
			return fmt.Errorf("Unhandled kind for %q: %v", path, v.Kind())
		}
	})

	if err != nil {
		glog.Fatalf("unexpected error finding dependencies %v", err)
	}

	return dependencies
}
