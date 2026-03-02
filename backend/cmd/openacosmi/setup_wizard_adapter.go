package main

// setup_wizard_adapter.go — 桥接 gateway.WizardPrompter → tui.WizardPrompter
//
// 使 cmd 层的 SetupChannels/SetupSkills/SetupInternalHooks
// 能够通过 gateway 的 RPC prompter 工作。
// 此文件在 cmd 层，不产生 import cycle。

import (
	"fmt"

	"github.com/openacosmi/claw-acismi/internal/gateway"
	"github.com/openacosmi/claw-acismi/internal/tui"
)

// gatewayToTuiPrompter 将 gateway.WizardPrompter 适配为 tui.WizardPrompter。
type gatewayToTuiPrompter struct {
	inner gateway.WizardPrompter
}

// newGatewayToTuiAdapter 创建适配器。
func newGatewayToTuiAdapter(p gateway.WizardPrompter) tui.WizardPrompter {
	return &gatewayToTuiPrompter{inner: p}
}

func (a *gatewayToTuiPrompter) Intro(title string) {
	_ = a.inner.Intro(title)
}

func (a *gatewayToTuiPrompter) Outro(message string) {
	_ = a.inner.Outro(message)
}

func (a *gatewayToTuiPrompter) Note(message, title string) {
	_ = a.inner.Note(message, title)
}

func (a *gatewayToTuiPrompter) Select(message string, options []tui.PromptOption, initialValue string) (string, error) {
	gwOpts := make([]gateway.WizardStepOption, len(options))
	for i, o := range options {
		gwOpts[i] = gateway.WizardStepOption{Value: o.Value, Label: o.Label, Hint: o.Hint}
	}
	val, err := a.inner.Select(message, gwOpts, initialValue)
	if err != nil {
		return "", err
	}
	return fmt.Sprint(val), nil
}

func (a *gatewayToTuiPrompter) MultiSelect(message string, options []tui.PromptOption, initialValues []string) ([]string, error) {
	gwOpts := make([]gateway.WizardStepOption, len(options))
	for i, o := range options {
		gwOpts[i] = gateway.WizardStepOption{Value: o.Value, Label: o.Label, Hint: o.Hint}
	}
	gwInitials := make([]interface{}, len(initialValues))
	for i, v := range initialValues {
		gwInitials[i] = v
	}
	vals, err := a.inner.MultiSelect(message, gwOpts, gwInitials)
	if err != nil {
		return nil, err
	}
	result := make([]string, len(vals))
	for i, v := range vals {
		result[i] = fmt.Sprint(v)
	}
	return result, nil
}

func (a *gatewayToTuiPrompter) TextInput(message, placeholder, initial string, _ func(string) string) (string, error) {
	return a.inner.Text(message, placeholder, initial, false)
}

func (a *gatewayToTuiPrompter) Confirm(message string, initial bool) (bool, error) {
	return a.inner.Confirm(message, initial)
}
