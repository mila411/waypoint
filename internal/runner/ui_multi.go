// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package runner

import (
	"io"

	"github.com/hashicorp/waypoint-plugin-sdk/terminal"
)

// multiUI mirrors UI operations to multiple UIs.
type multiUI struct {
	UIs []terminal.UI
}

func (u *multiUI) Close() error {
	for _, u := range u.UIs {
		if c, ok := u.(io.Closer); ok {
			c.Close()
		}
	}

	return nil
}

func (u *multiUI) Input(input *terminal.Input) (string, error) {
	return "", terminal.ErrNonInteractive
}

func (u *multiUI) Interactive() bool {
	return false
}

func (u *multiUI) Output(msg string, raw ...interface{}) {
	for _, u := range u.UIs {
		u.Output(msg, raw...)
	}
}

func (u *multiUI) NamedValues(tvalues []terminal.NamedValue, opts ...terminal.Option) {
	for _, u := range u.UIs {
		u.NamedValues(tvalues, opts...)
	}
}

func (u *multiUI) OutputWriters() (stdout io.Writer, stderr io.Writer, err error) {
	return u.UIs[0].OutputWriters()
}

func (u *multiUI) Table(tbl *terminal.Table, opts ...terminal.Option) {
	for _, u := range u.UIs {
		u.Table(tbl, opts...)
	}
}

func (u *multiUI) Status() terminal.Status {
	var s []terminal.Status
	for _, u := range u.UIs {
		s = append(s, u.Status())
	}

	return &multiUIStatus{s}
}

type multiUIStatus struct {
	s []terminal.Status
}

func (u *multiUIStatus) Update(msg string) {
	for _, s := range u.s {
		s.Update(msg)
	}
}

func (u *multiUIStatus) Step(status string, msg string) {
	for _, s := range u.s {
		s.Step(status, msg)
	}
}

func (u *multiUIStatus) Close() error {
	for _, s := range u.s {
		s.Close()
	}

	return nil
}

type multiUISGStep struct {
	steps []terminal.Step
}

func (u *multiUISGStep) TermOutput() io.Writer {
	var ws []io.Writer
	for _, s := range u.steps {
		ws = append(ws, s.TermOutput())
	}

	return io.MultiWriter(ws...)
}

func (u *multiUISGStep) Update(str string, args ...interface{}) {
	for _, s := range u.steps {
		s.Update(str, args...)
	}
}

func (u *multiUISGStep) Status(status string) {
	for _, s := range u.steps {
		s.Update(status)
	}
}

func (u *multiUISGStep) Done() {
	for _, s := range u.steps {
		s.Done()
	}
}

func (u *multiUISGStep) Abort() {
	for _, s := range u.steps {
		s.Abort()
	}
}

type multiUISG struct {
	sgs []terminal.StepGroup
}

func (u *multiUISG) Add(str string, args ...interface{}) terminal.Step {
	var steps []terminal.Step
	for _, sg := range u.sgs {
		steps = append(steps, sg.Add(str, args...))
	}

	return &multiUISGStep{steps}
}

func (u *multiUISG) Wait() {
	for _, sg := range u.sgs {
		sg.Wait()
	}
}

func (u *multiUI) StepGroup() terminal.StepGroup {
	var sgs []terminal.StepGroup
	for _, u := range u.UIs {
		sgs = append(sgs, u.StepGroup())
	}

	return &multiUISG{sgs}
}

var (
	_ terminal.UI     = (*multiUI)(nil)
	_ terminal.Status = (*multiUIStatus)(nil)
)
