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
	"regexp"
)

const (
	LSN_RESULT_REGEXP = `([0-9a-fA-F]+,[0-9a-fA-F]+/[0-9a-fA-F]+)`
	LSN_VALUE_REGEXP  = `[0-9a-fA-F]+`
)

type LSN struct {
	PodName string
	PodIP   string
	Left    int
	Middle  int
	Right   int
}

func LSNZero() LSN {
	return LSN{
		PodName: "",
		PodIP:   "",
		Left:    0,
		Middle:  0,
		Right:   0,
	}
}

func (lsn LSN) String() string {
	return fmt.Sprintf("%d,%d/%d", lsn.Left, lsn.Middle, lsn.Right)
}

func (lsn LSN) CompareTo(another LSN) int {
	result := CompareInt(lsn.Left, another.Left)
	if result != 0 {
		return result
	}
	result = CompareInt(lsn.Middle, another.Middle)
	if result != 0 {
		return result
	}
	return CompareInt(lsn.Right, another.Right)
}

func (lsn LSN) CompareIP(that LSN) int {
	result := 0
	if lsn.PodIP != that.PodIP {
		if lsn.PodIP == "" {
			result = -1
		} else if that.PodIP == "" {
			result = 1
		} else {
			thisArray := IPToIntArray(lsn.PodIP)
			thatArray := IPToIntArray(that.PodIP)
			for i := 0; i < len(thisArray); i++ {
				r := CompareInt(thisArray[i], thatArray[i])
				if r != 0 {
					result = r
					break
				}
			}
		}
	}
	return result
}

func CompareInt(this, that int) int {
	if this > that {
		return 1
	} else if this < that {
		return -1
	} else {
		return 0
	}
}

func ParseLSN(podName, podIP, val string) LSN {
	var left, middle, right int
	reg := regexp.MustCompile(LSN_RESULT_REGEXP)
	lsnVal := reg.FindString(val)
	if lsnVal != "" {
		reg = regexp.MustCompile(LSN_VALUE_REGEXP)
		params := reg.FindAllString(lsnVal, -1)
		if len(params) == 3 {
			left = Hex2Dec(params[0])
			middle = Hex2Dec(params[1])
			right = Hex2Dec(params[2])
		}
	}
	return LSN{
		PodName: podName,
		PodIP:   podIP,
		Left:    left,
		Middle:  middle,
		Right:   right,
	}
}
