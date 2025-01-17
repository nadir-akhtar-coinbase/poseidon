package main

import (
	"bytes"
	"fmt"
	"math"
	"math/big"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
	"github.com/triplewz/poseidon"
)

type constantsStr struct {
	C [][]string // compRoundConstants
	S [][]string // sparseMatrix
	M [][][]string // MDS matrix
	P [][][]string // preSparseMatrix
}

func main() {
	levels := 2 // to set to 15
	startingLevel := 2
	field := 1
	sbox := 0
	fieldSize := fr.Bits
	alpha := poseidon.Alpha // TODO check value for different curves
	securityLevel := poseidon.SecurityLevel

	cs := constantsStr{
		C: make([][]string, levels),
		S: make([][]string, levels),
		M: make([][][]string, levels),
		P: make([][][]string, levels),
	}

	for width := startingLevel; width < levels + startingLevel; width++ {
		args := []string{
			"./generator/generate_params_poseidon.sage",
			strconv.Itoa(field),
			strconv.Itoa(sbox),
			strconv.Itoa(fieldSize),
			strconv.Itoa(width),
			strconv.Itoa(alpha),
			strconv.Itoa(securityLevel),
			fmt.Sprintf("0x%s", fr.Modulus().Text(16)),
		}
		fmt.Printf("Executing 'sage %s'\n", strings.Join(args, " "))
		out, err := exec.Command("sage", args...).Output()
		if err != nil {
			panic(err)
		}

		mdsMatrixStart := bytes.Index(out, []byte("MDS matrix:"))
		mdsMatrixEnd := mdsMatrixStart + bytes.Index(out[mdsMatrixStart:], []byte("]]"))
		mdsMatrixString := string(out[mdsMatrixStart+12 : mdsMatrixEnd+2])
		hexStringRegexp := regexp.MustCompile(`'0x[0-9a-fA-F]+'`)

		var mdsMatrix poseidon.Matrix[*fr.Element]
		mdsMatrixStrings := hexStringRegexp.FindAllString(mdsMatrixString, -1)
		mdsWidth := int(math.Round(math.Sqrt(float64(len(mdsMatrixStrings)))))
		for i := 0; i < mdsWidth; i++ {
			mdsMatrix = append(mdsMatrix, make([]*fr.Element, mdsWidth))
			for j := 0; j < mdsWidth; j++ {
				match := mdsMatrixStrings[j*mdsWidth+i]
				mdsValue, ok := new(big.Int).SetString(match[3:len(match)-1], 16)
				if !ok {
					panic(fmt.Sprintf("could not parse hex value: %s", match))
				}
				mdsMatrix[i][j] = new(fr.Element).SetBigInt(mdsValue)
			}
		}

		constants, err := poseidon.GenPoseidonConstants[*fr.Element](width, field, sbox, true, mdsMatrix)
		if err != nil {
			panic(err)
		}

		level := width - startingLevel

		// C
		cs.C[level] = make([]string, len(constants.CompRoundConsts))
		for i, e := range constants.CompRoundConsts {
			cs.C[level][i] = e.ToBigIntRegular(new(big.Int)).Text(16)
		}

		// S
		for _, e := range constants.Sparse {
			for _, w := range e.WHat {
				cs.S[level] = append(cs.S[level], w.ToBigIntRegular(new(big.Int)).Text(16))
			}

			for _, v := range e.V {
				cs.S[level] = append(cs.S[level], v.ToBigIntRegular(new(big.Int)).Text(16))
			}
		}

		// M
		cs.M[level] = make([][]string, len(mdsMatrix))
		for i, e := range mdsMatrix {
			cs.M[level][i] = make([]string, len(e))
			for j, f := range e {
				cs.M[level][i][j] = f.ToBigIntRegular(new(big.Int)).Text(16)
			}
		}

		// P
		cs.P[level] = make([][]string, len(constants.PreSparse))
		for i, e := range constants.PreSparse {
			cs.P[level][i] = make([]string, len(e))
			for j, f := range e {
				cs.P[level][i][j] = f.ToBigIntRegular(new(big.Int)).Text(16)
			}
		}


	}

	var b bytes.Buffer
	err := GenerateTemplate(&b, &cs)
	if err != nil {
		panic(err)
	}
	fmt.Println(b.String())
}
