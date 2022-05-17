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
	"bytes"
	"strings"
	"time"

	"k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	EXE_RETRY_LIMIT    = 5
	EXE_RETRY_INTERVAL = 3
)

/*
执行Pod内部命令的结构体
*/
type Executor struct {
	KubeClient *kubernetes.Clientset
	KubeConfig *rest.Config
	Namespace  string
	PodName    string
	Container  string
}

/*
执行返回结果
*/
type ExecutorResult struct {
	Stdout bytes.Buffer
	Stderr bytes.Buffer
	Stdin  bytes.Reader
}

func NewExecutor() Executor {
	return Executor{
		KubeConfig: ctrl.GetConfigOrDie(),
		KubeClient: kubernetes.NewForConfigOrDie(ctrl.GetConfigOrDie()),
	}
}

/*
选取执行命令的容器
*/
func (e *Executor) Select(namespace, podName, container string) *Executor {
	exec := &Executor{
		KubeConfig: e.KubeConfig,
		KubeClient: e.KubeClient,
		Namespace:  namespace,
		PodName:    podName,
		Container:  container,
	}

	return exec
}

/*
在容器中执行命令
*/
func (e *Executor) Exec(command string) (string, string, error) {
	cmd := []string{
		"sh",
		"-c",
		command,
	}
	const tty = false
	req := e.KubeClient.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(e.PodName).
		Namespace(e.Namespace).SubResource("exec").Param("container", e.Container)
	req.VersionedParams(
		&v1.PodExecOptions{
			Command: cmd,
			Stdin:   false,
			Stdout:  true,
			Stderr:  true,
			TTY:     tty,
		},
		scheme.ParameterCodec,
	)

	var stdout, stderr bytes.Buffer
	exec, err := remotecommand.NewSPDYExecutor(e.KubeConfig, "POST", req.URL())
	if err != nil {
		return "", "", err
	}
	for i := 0; i < EXE_RETRY_LIMIT; i++ {
		err = exec.Stream(remotecommand.StreamOptions{
			Stdin:  nil,
			Stdout: &stdout,
			Stderr: &stderr,
		})
		if err != nil {
			time.Sleep(time.Second * EXE_RETRY_INTERVAL)
		} else {
			break
		}
	}
	if err != nil {
		return "", "", err
	}

	return strings.TrimSpace(stdout.String()), strings.TrimSpace(stderr.String()), err
}
