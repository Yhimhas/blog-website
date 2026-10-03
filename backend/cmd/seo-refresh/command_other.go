//go:build !linux

package main

import "os/exec"

func configureCommand(command *exec.Cmd) {}
