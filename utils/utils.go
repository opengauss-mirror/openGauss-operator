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
	"encoding/json"
	"fmt"
	"math"
	"net"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

const (
	IP_REG            = `([\d]{1,3})\.([\d]{1,3})\.([\d]{1,3})\.([\d]{1,3})`
	POD_NAME_EXP      = `pod-([\d]{1,3})`
	RESOURCE_EXP      = `([0-9.]+)(Ki|K|Mi|Gi|M|G|Ti|T)`
	DEFAULT_TIMEOUT   = 5
	RETRY_INTERVAL    = 10
	RETRY_LIMIT       = 30
	TIME_FORMAT       = "2006-01-02 15:04:05"
	DEFAULT_DELIMITER = ","
)

func IPToIntArray(ip string) []int {
	array := make([]int, 0)
	reg := regexp.MustCompile(IP_REG)
	params := reg.FindStringSubmatch(ip)

	length := len(params)
	if length == 5 {
		for i := 1; i < length; i++ {
			val, _ := strconv.Atoi(params[i])
			array = append(array, val)
		}
	}
	return array

}

func Hex2Dec(val string) int {
	n, err := strconv.ParseUint(val, 16, 32)
	if err != nil {
		fmt.Println(err)
	}
	return int(n)
}

func ValidateResource(val string) bool {
	if val == "" {
		return true
	}
	_, err := resource.ParseQuantity(val)
	if err != nil {
		return false
	}
	return true
}
func ValidateIp(ipval string) bool {
	IP := net.ParseIP(ipval)
	return IP != nil && IP.IsGlobalUnicast()
}
func CompareResource(resources corev1.ResourceList, resourceName corev1.ResourceName, expectVal string) bool {
	actualResource := resources[resourceName]
	actualResourceStr := actualResource.String()
	return CompareQuantity(actualResourceStr, expectVal) == 0
}
func CompareQuantity(val1, val2 string) int {
	q1, _ := resource.ParseQuantity(val1)
	q2, _ := resource.ParseQuantity(val2)
	return q1.Cmp(q2)
}

/*
将String格式化为K8S的资源值
*/
func FormatResourceToParamVal(val string) string {
	reg := regexp.MustCompile(RESOURCE_EXP)
	params := reg.FindStringSubmatch(val)
	v, _ := strconv.ParseFloat(params[1], 64)
	u := params[2]

	if strings.HasSuffix(u, "i") {
		u = strings.Replace(u, "i", "B", 1)
	} else {
		u = u + "B"
	}
	return fmt.Sprintf("%.0f%s", v, u)
}

/*
根据给定的值和比例，计算资源值，用于某些数据库内存参数的计算
方法参数：
	val：资源输入值
	factor：折算比例
	defaultVal：默认值
方法逻辑：
	计算结果=val*factor
	如果计算结果小于默认值，则返回默认值，否则返回计算结果
*/
func CalculateResourceByPercentage(val string, factor float64, defaultVal string) string {
	reg := regexp.MustCompile(RESOURCE_EXP)
	params := reg.FindStringSubmatch(val)
	v, _ := strconv.ParseFloat(params[1], 64)
	u := params[2]
	floatVal := v * factor
	if math.Ceil(floatVal) != floatVal {
		floatVal = 1024 * floatVal
		if strings.Index(u, "G") >= 0 {
			u = strings.Replace(u, "G", "M", 1)
		} else if strings.Index(u, "T") >= 0 {
			u = strings.Replace(u, "T", "G", 1)
		}
	}

	format := "%.0f%s"
	result := fmt.Sprintf(format, floatVal, u)

	if defaultVal != "" && CompareQuantity(result, defaultVal) < 0 {
		result = defaultVal
	}
	return result
}

func MergeMaps(maps ...map[string]string) map[string]string {
	result := make(map[string]string)
	for _, m := range maps {
		for k, v := range m {
			result[k] = v
		}
	}
	return result
}
func CompareMaps(this, that map[string]string) bool {
	if (this == nil || len(this) == 0) && (that == nil || len(that) == 0) {
		return true
	}
	return reflect.DeepEqual(this, that)
}
func GetChangedMap(oldMap, newMap map[string]string) map[string]string {
	changed := make(map[string]string)
	if len(oldMap) == 0 {
		changed = newMap
	} else if !CompareMaps(oldMap, newMap) {
		for key, value := range newMap {
			oldVal, exist := oldMap[key]
			if !exist || oldVal != value {
				changed[key] = value
			}
		}
	}
	return changed
}
func Ping(target string, port int32) error {
	timeout := time.Duration(time.Second * DEFAULT_TIMEOUT)
	_, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", target, port), timeout)
	if err != nil {
		return err
	}
	return nil
}

func MapToString(param map[string]string) string {
	dataType, _ := json.Marshal(param)
	return string(dataType)
}

func StringArrayToString(array []string) string {
	var buffer strings.Builder
	for _, str := range array {
		if buffer.Len() > 0 {
			buffer.WriteString("\n")
		}
		buffer.WriteString(str)
	}
	return buffer.String()
}

func StringToSet(value string) Set {
	return StringToSetByDelimiter(value, DEFAULT_DELIMITER)
}

func StringToSetByDelimiter(value, delimiter string) Set {
	result := NewSet()
	if strings.TrimSpace(value) != "" {
		array := strings.Split(value, delimiter)
		for _, val := range array {
			v := strings.TrimSpace(val)
			if v != "" {
				result.Add(v)
			}
		}
	}
	return result
}

func HashCode(vals ...string) int {
	code := 0
	for _, v := range vals {
		bytes := []byte(v)
		for _, b := range bytes {
			code = 31*code + int(b)
		}
	}
	if code < 0 {
		code = code * (-1)
	}
	return code
}

func CalculateSyncCount(localCount, remoteCount int) int {
	count := 0
	if remoteCount > 0 {
		count = localCount
	} else if localCount >= 2 {
		count = localCount / 2
	}
	return count
}
