//go:build !windows

package main

func inspectForeground() hudTarget {
	return hudTarget{Error: "kill switch is only available on Windows"}
}

func killForegroundPID(target hudTarget) hudTarget {
	target.OK = false
	if target.Error == "" {
		target.Error = "kill switch is only available on Windows"
	}
	return target
}

func hudInspectFromService() hudTarget {
	return inspectForeground()
}

func hudKillFromService() hudTarget {
	return killForegroundPID(inspectForeground())
}
