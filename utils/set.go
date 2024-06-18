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
	"fmt"
	"reflect"
	"strings"
)

type void struct{}

var member void

/*
集合类型，仅支持String
*/
type Set struct {
	Map map[string]void
}

func NewSet() Set {
	return Set{
		Map: make(map[string]void),
	}
}
func NewSetFromSet(set Set) Set {
	newSet := Set{
		Map: make(map[string]void),
	}
	newSet.AddAllFromSet(set)
	return newSet
}

func NewSetFromArray(array []string) Set {
	newSet := Set{
		Map: make(map[string]void),
	}
	newSet.AddAllFromArray(array)
	return newSet
}
func (set Set) Add(val string) {
	if val != "" {
		set.Map[val] = member
	}
}

func (set Set) Contains(val string) bool {
	_, exist := set.Map[val]
	return exist
}

func (set Set) ContainsAll(another Set) bool {
	for key, _ := range another.Map {
		if !set.Contains(key) {
			return false
		}
	}
	return true
}

func (set Set) Remove(val string) {
	delete(set.Map, val)
}

func (set Set) RemoveAll() {
	set.Map = make(map[string]void)
}

func (set Set) AddAllFromArray(array []string) {
	for _, val := range array {
		set.Add(val)
	}
}

func (set Set) AddAllFromSet(another Set) {
	for k, _ := range another.Map {
		set.Add(k)
	}
}

func (set Set) RemoveAllFromSet(another Set) {
	for k, _ := range another.Map {
		set.Remove(k)
	}
}

/*
比较两个set是否相同
Size相同，且内容相同
*/
func (set Set) Equals(another Set) bool {
	return set.Size() == another.Size() && reflect.DeepEqual(set.Map, another.Map)
}

/*
比较当前set与传入的another set不同
参数： 传入的set
返回值：beyond : 当前set与传入的set比较，多出的元素
       missing： 当前set与传入的set比较，缺少的元素
*/
func (set Set) DiffTo(another Set) ([]string, []string) {
	beyond := make([]string, 0)
	missing := make([]string, 0)
	if set.Equals(another) {
		return beyond, missing
	}
	if another.IsEmpty() {
		return set.ToArray(), missing
	} else if set.IsEmpty() {
		return beyond, another.ToArray()
	} else {
		this := NewSetFromSet(set)
		that := NewSetFromSet(another)
		for k, _ := range this.Map {
			if that.Contains(k) {
				that.Remove(k)
			}
		}
		if !that.IsEmpty() {
			for k, _ := range that.Map {
				if this.Contains(k) {
					this.Remove(k)
				}
			}
		}
		beyond = this.ToArray()
		missing = that.ToArray()
	}
	return beyond, missing
}

func (set Set) String() string {
	var buf strings.Builder
	for k, _ := range set.Map {
		if buf.Len() > 0 {
			buf.WriteString(", ")
		}
		buf.WriteString(k)
	}
	return fmt.Sprintf("[%s]", buf.String())
}

func (set Set) ToArray() []string {
	ipArray := make([]string, 0)
	for k, _ := range set.Map {
		ipArray = append(ipArray, k)
	}
	return ipArray
}

func (set Set) Size() int {
	return len(set.Map)
}

func (set Set) IsEmpty() bool {
	return set.Size() == 0
}
