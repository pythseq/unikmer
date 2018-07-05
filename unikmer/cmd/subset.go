// Copyright © 2018 Wei Shen <shenwei356@gmail.com>
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package cmd

import (
	"fmt"
	"io"
	"runtime"
	"strings"

	"github.com/shenwei356/unikmer"
	"github.com/shenwei356/xopen"
	"github.com/spf13/cobra"
	boom "github.com/tylertreat/BoomFilters"
)

// subsetCmd represents
var subsetCmd = &cobra.Command{
	Use:   "subset",
	Short: "extract smaller kmers from binary file",
	Long: `extract smaller kmers from binary file

Attention:
  - It's faster than re-counting from sequence file but in cost of losing
    few ( <= (K-k)*2 ) kmers in the ends of sequence and its reverse complement
    sequence.

`,
	Run: func(cmd *cobra.Command, args []string) {
		opt := getOptions(cmd)
		runtime.GOMAXPROCS(opt.NumCPUs)
		files := getFileList(args)

		if len(files) > 1 {
			checkError(fmt.Errorf("no more than one file should be given"))
		}

		outFile := getFlagString(cmd, "out-prefix")
		k := getFlagPositiveInt(cmd, "kmer-len")
		if k > 32 {
			checkError(fmt.Errorf("k > 32 not supported"))
		}
		hint := getFlagPositiveInt(cmd, "esti-kmer-num")

		file := files[0]

		if !isStdin(file) && !strings.HasSuffix(file, extDataFile) {
			log.Errorf("input should be stdin or %s file", extDataFile)
			return
		}

		var err error
		var infh *xopen.Reader

		infh, err = xopen.Ropen(file)
		checkError(err)
		defer infh.Close()

		var reader *unikmer.Reader
		reader, err = unikmer.NewReader(infh)
		checkError(err)

		if k >= reader.K {
			log.Errorf("k (%d) should be small than k size (%d) of %s", k, reader.K, file)
			return
		}

		if !isStdout(outFile) {
			outFile += extDataFile
		}

		outfh, err := xopen.WopenGzip(outFile)
		checkError(err)
		defer outfh.Close()

		writer := unikmer.NewWriter(outfh, k)

		sbf := boom.NewScalableBloomFilter(uint(hint), 0.01, 0.8)

		var kcode, kcode2 unikmer.KmerCode
		var mer []byte
		for {
			kcode, err = reader.Read()
			if err != nil {
				if err == io.EOF {
					break
				}
				checkError(err)
			}

			mer = kcode.Bytes()
			mer = mer[0:k]

			kcode2, err = unikmer.NewKmerCode(mer)
			if err != nil {
				checkError(fmt.Errorf("encoding '%s': %s", mer, err))
			}

			if !sbf.Test(mer) {
				sbf.Add(mer)
				checkError(writer.Write(kcode2))
			}
		}
	},
}

func init() {
	RootCmd.AddCommand(subsetCmd)

	subsetCmd.Flags().StringP("out-prefix", "o", "-", `out file prefix ("-" for stdout)`)
	subsetCmd.Flags().IntP("kmer-len", "k", 0, "kmer length")
	subsetCmd.Flags().IntP("esti-kmer-num", "n", 100000000, "estimated kmer num length (for initializing Bloom Filter)")
}
