package bilibili

import (
	"os/exec"
	"strconv"
	"time"
)

func configureProcess(c *exec.Cmd) {
	c.Cancel = func() error {
		if c.Process == nil {
			return nil
		}
		err := exec.Command("taskkill", "/PID", strconv.Itoa(c.Process.Pid), "/T", "/F").Run()
		if err != nil {
			return c.Process.Kill()
		}
		return nil
	}
	c.WaitDelay = time.Second
}
