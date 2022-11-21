/*
Copyright © 2022 Sergio Marin <@highercomve>
*/
package cmd

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/highercomve/tartool/utils"
	"github.com/spf13/cobra"
	"github.com/ulikunitz/xz"
)

// mergeCmd represents the merge command
var mergeCmd = &cobra.Command{
	Use:   "merge [flags] [file1] [file2] ...",
	Short: "merge several tar file into one.",
	Long:  `merge several tar file into one.`,
	RunE:  tarCatfunc,
}

func init() {
	rootCmd.AddCommand(mergeCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// mergeCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:

	mergeCmd.Flags().BoolP("compress", "c", true, "compress output")
	mergeCmd.Flags().StringP("format", "f", "xz", "compress format (xz, gzip)")
	mergeCmd.Flags().StringP("output", "o", "-", "output file (default standard output)")
	mergeCmd.Flags().StringP("inputformat", "l", "", "compress format of the input (xz, gzip, none)")
}

func tarCatfunc(cmd *cobra.Command, args []string) (err error) {
	var out io.Writer

	compress, err := cmd.Flags().GetBool("compress")
	if err != nil {
		return err
	}
	format, err := cmd.Flags().GetString("format")
	if err != nil {
		return err
	}
	inputformat, err := cmd.Flags().GetString("inputformat")
	if err != nil {
		return err
	}
	output, err := cmd.Flags().GetString("output")
	if err != nil {
		return err
	}

	if len(args) == 0 {
		return fmt.Errorf("arguments need it")
	}

	switch output {
	case "":
		return fmt.Errorf("missing output file, -h/--help to see how to use the command")
	case "-":
		out = os.Stdout
	default:
		out, err = utils.CreateNewOutput(output)
		if err != nil {
			return fmt.Errorf("failed to open %s: %v", output, err)
		}
	}

	var cout *tar.Writer
	if compress {
		switch format {
		case "xz":
			cout, err := xz.NewWriter(out)
			if err != nil {
				return err
			}
			out = cout
		case "gzip":
			cout, err := gzip.NewWriterLevel(out, gzip.BestCompression)
			if err != nil {
				return err
			}
			out = cout
		default:
			return fmt.Errorf("compression format not available: %s", format)
		}
	}

	tw := tar.NewWriter(out)
	if stdin, rc, err := utils.OpenTarBuffer(os.Stdin, inputformat); err == nil {
		if err := utils.AddTarFromWriter(tw, stdin, rc); err != nil {
			return fmt.Errorf("failed to merge %s: %v", "input", err)
		}
	} else if !strings.Contains(err.Error(), "empty") {
		return fmt.Errorf("failed to merge %s: %v", "input", err)
	}

	for _, v := range args {
		if err := utils.AddTar(tw, v); err != nil {
			return fmt.Errorf("failed to merge %s: %s", v, err.Error())
		}
	}

	if err = tw.Close(); err != nil {
		return fmt.Errorf("failed to close compressed writer, %v", err)
	}

	if cout != nil {
		if err = cout.Close(); err != nil {
			return fmt.Errorf("failed to close compressed writer, %v", err)
		}
	}

	return err
}
