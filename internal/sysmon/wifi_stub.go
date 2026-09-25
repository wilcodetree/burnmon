//go:build !windows

package sysmon

func readWifi() WifiSample { return WifiSample{} }
