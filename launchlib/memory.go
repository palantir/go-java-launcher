package launchlib

import (
	"io"
	"io/fs"
	"math"
	"path/filepath"
	"strconv"

	"github.com/pkg/errors"
)

const (
	memGroupName = "memory"
	memLimitName = "memory.limit_in_bytes"
)

type RAMPercenter interface {
	RAMPercent() (float64, error)
}

type ChainedRAMPercenter struct {
	delegates []RAMPercenter
}

func NewChainedRAMPercenter(delegates ...RAMPercenter) RAMPercenter {
	return ChainedRAMPercenter{
		delegates: delegates,
	}
}

func (c ChainedRAMPercenter) RAMPercent() (float64, error) {
	for _, percenter := range c.delegates {
		p, err := percenter.RAMPercent()
		if err != nil {
			// log and move on
		}
		return p, nil
	}
	return 0, errors.New("failed to get RAM percentage from all configured delegates")
}

type StaticRAMPercent struct {
	percent float64
}

func NewStaticRAMPercent(percent float64) RAMPercenter {
	return StaticRAMPercent{
		percent: percent,
	}
}

func (s StaticRAMPercent) RAMPercent() (float64, error) {
	return s.percent, nil
}

const (
	lowerBound = 75
	upperBound = 95
	growthRate = 1
	midpoint   = 4294967296
	sharpness  = 1
)

var scalingFunc = genlog(lowerBound, upperBound, growthRate, midpoint, sharpness)

type ScalingRAMPercent struct {
	pather CGroupPather
	fs     fs.FS
}

func NewScalingRAMPercenter(filesystem fs.FS) RAMPercenter {
	return ScalingRAMPercent{
		fs:     filesystem,
		pather: NewCGroupV1Pather(filesystem),
	}
}

func (s ScalingRAMPercent) RAMPercent() (float64, error) {
	// read limit from cgroup
	memoryCGroupPath, err := s.pather.GetPath(memGroupName)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get memory cgroup path")
	}

	memLimitFilepath := filepath.Join(memoryCGroupPath, memLimitName)
	memLimitFile, err := s.fs.Open(convertToFSPath(memLimitFilepath))
	if err != nil {
		return 0, errors.Wrapf(err, "unable to open cpu.shares at expected location: %s", memLimitFilepath)
	}
	memLimitBytes, err := io.ReadAll(memLimitFile)
	if err != nil {
		return 0, errors.Wrapf(err, "unable to read cpu.shares")
	}
	memLimit, err := strconv.Atoi(string(memLimitBytes))
	if err != nil {
		return 0, errors.New("unable to convert mem.limit_in_bytes value to expected type")
	}

	return scalingFunc(float64(memLimit)), nil
}

func genlog(min float64, max float64, growthRate float64, midpoint float64, v float64) func(float64) float64 {
	return func(in float64) float64 {
		// https://en.wikipedia.org/wiki/Generalised_logistic_function#Definition
		return min + (max-min)/(math.Pow(1+math.Pow(math.E, -1*growthRate*(in-midpoint)), 1/v))
	}
}