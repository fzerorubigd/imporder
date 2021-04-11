package main

import (
	"golang.org/x/tools/go/analysis/singlechecker"

	"github.com/fzerorubigd/imporder/pkg/imporder"
)

func main() {
	singlechecker.Main(imporder.NewImportAnalyzer())
}
