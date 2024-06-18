/*
Copyright (c) 2021 opensource@cmbc.com.cn
OpenGauss Operator is licensed under Mulan PSL v2.
You can use this software according to the terms and conditions of the Mulan PSL v2.
You may obtain a copy of Mulan PSL v2 at:
         http://license.coscl.org.cn/MulanPSL2
THIS SOFTWARE IS PROVIDED ON AN "AS IS" BASIS, WITHOUT WARRANTIES OF ANY KIND,
EITHER EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO NON-INFRINGEMENT,
MERCHANTABILITY OR FIT FOR A PARTICULAR PURPOSE.
See the Mulan PSL v2 for more details.
*/

package utils

import (
	"os"
	"strconv"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const (
	ENV_NAME_KEY  = "INSTANCE_NAME"
	ENV_TOTAL_KEY = "TOTAL_COUNT"
)

type WatchScopePredicate struct {
	WatchNamespaces   Set
	ExcludeNamespaces Set
}

func NewWatchScopePredicate(watchNamespaces, excludeNamespace string) predicate.Predicate {
	return WatchScopePredicate{
		WatchNamespaces:   StringToSet(watchNamespaces),
		ExcludeNamespaces: StringToSet(excludeNamespace),
	}
}

func (wsp WatchScopePredicate) Create(e event.CreateEvent) bool {
	return wsp.checkNamespace(e.Object.GetNamespace())
}

func (wsp WatchScopePredicate) Delete(e event.DeleteEvent) bool {
	return wsp.checkNamespace(e.Object.GetNamespace())
}

func (wsp WatchScopePredicate) Update(e event.UpdateEvent) bool {
	return wsp.checkNamespace(e.ObjectNew.GetNamespace())
}

func (wsp WatchScopePredicate) Generic(e event.GenericEvent) bool {
	return wsp.checkNamespace(e.Object.GetNamespace())
}

func (wsp WatchScopePredicate) checkNamespace(namespace string) bool {
	if !wsp.WatchNamespaces.IsEmpty() {
		return wsp.WatchNamespaces.Contains(namespace)
	} else if !wsp.ExcludeNamespaces.IsEmpty() {
		return !wsp.ExcludeNamespaces.Contains(namespace)
	} else {
		return true
	}
}

type HashCodePredicate struct {
	Start int
	End   int
	Total int
}

func NewHashCodePredicate(instanceRange int) predicate.Predicate {
	instanceCount := 0
	totalVal := os.Getenv(ENV_TOTAL_KEY)
	if totalVal != "" {
		if v, e := strconv.Atoi(totalVal); e == nil {
			instanceCount = v
		}
	}

	start := 0
	end := 0
	total := 0
	if instanceCount > 1 {
		instanceIndex := getIndex()
		start = instanceIndex * instanceRange
		end = (instanceIndex + 1) * instanceRange
		total = instanceCount * instanceRange
	}

	return HashCodePredicate{
		Start: start,
		End:   end,
		Total: total,
	}
}

func getIndex() int {
	name := os.Getenv(ENV_NAME_KEY)
	index := 0
	if strIndex := strings.LastIndex(name, "-"); strIndex > 0 {
		subStr := name[strIndex+1:]
		if i, e := strconv.Atoi(subStr); e == nil {
			index = i
		}
	}
	return index
}

func (hcp HashCodePredicate) Create(e event.CreateEvent) bool {
	return hcp.checkHash(e.Object.GetNamespace(), e.Object.GetName())
}
func (hcp HashCodePredicate) Delete(e event.DeleteEvent) bool {
	return hcp.checkHash(e.Object.GetNamespace(), e.Object.GetName())
}

func (hcp HashCodePredicate) Update(e event.UpdateEvent) bool {
	return hcp.checkHash(e.ObjectNew.GetNamespace(), e.ObjectNew.GetName())
}

func (hcp HashCodePredicate) Generic(e event.GenericEvent) bool {
	return hcp.checkHash(e.Object.GetNamespace(), e.Object.GetName())
}

func (hcp HashCodePredicate) checkHash(namespace, name string) bool {
	if hcp.Total == 0 {
		return true
	}
	hashCode := HashCode(namespace, name)
	v := hashCode % hcp.Total
	return v >= hcp.Start && v < hcp.End
}
