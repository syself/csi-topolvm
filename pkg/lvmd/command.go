package lvmd

import (
	internalLvmdCommand "github.com/syself/csi-topolvm/internal/lvmd/command"
)

func Containerized(sw bool) {
	internalLvmdCommand.Containerized = sw
}
