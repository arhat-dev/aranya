package pod

import (
	"fmt"
	"path/filepath"
)

func podLogDir(logRootDir, namespace, podName, podUid string) string {
	return filepath.Join(logRootDir, fmt.Sprintf("%s_%s_%s", namespace, podName, podUid))
}

func containerLogFile(podLogDir, containerName string) string {
	return filepath.Join(podLogDir, containerName, "1.log")
}
