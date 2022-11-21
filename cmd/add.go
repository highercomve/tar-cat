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

	"github.com/highercomve/tartool/utils"
	"github.com/spf13/cobra"
	"github.com/ulikunitz/xz"
)

// addCmd represents the add command
var addCmd = &cobra.Command{
	Use:   "add [flags] [file1] [file2] ...",
	Short: "add files from a list to a tar file",
	Long:  `add files from a list to a tar file`,
	RunE:  tarAddFunc,
}

func init() {
	rootCmd.AddCommand(addCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// addCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	addCmd.Flags().StringP("input", "i", "", "tar file path used as input")
	addCmd.Flags().StringP("output", "o", "-", "tar file path used as output")
	addCmd.Flags().StringP("format", "f", "xz", "compress format (xz, gzip, none)")
	addCmd.Flags().StringP("inputformat", "l", "", "compress format of the input (xz, gzip, none)")
	addCmd.Flags().StringP("directory", "d", "", "change to directory DIR")
}

func tarAddFunc(cmd *cobra.Command, args []string) (err error) {
	var outputFile *os.File
	var out io.Writer

	input, err := cmd.Flags().GetString("input")
	if err != nil {
		return err
	}
	output, err := cmd.Flags().GetString("output")
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
	directory, err := cmd.Flags().GetString("directory")
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
		outputFile = os.Stdout
	default:
		outputFile, err = utils.CreateNewOutput(output)
		if err != nil {
			return fmt.Errorf("failed to open %s: %v", output, err)
		}
	}

	out = outputFile
	var cout io.WriteCloser
	switch format {
	case "xz":
		cout, err = xz.NewWriter(out)
		if err != nil {
			return err
		}
		out = cout
	case "gzip":
		cout, err = gzip.NewWriterLevel(out, gzip.BestCompression)
		if err != nil {
			return err
		}
		out = cout
	}

	tw := tar.NewWriter(out)

	for _, v := range args {
		if err := utils.AddFile(tw, v, directory); err != nil {
			return fmt.Errorf("failed to merge %s: %s", v, err.Error())
		}
	}

	if input != "" {
		if err := utils.AddFile(tw, input, directory); err != nil {
			return fmt.Errorf("failed to merge %s: %s", input, err.Error())
		}
	} else {
		if stdin, rc, err := utils.OpenTarBuffer(os.Stdin, inputformat); err == nil {
			if err := utils.AddTarFromBuffer(tw, stdin, rc); err != nil {
				return fmt.Errorf("failed to merge %s: %v", "stdin", err)
			}
		} else {
			return fmt.Errorf("failed to merge %s: %s", input, err.Error())
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
