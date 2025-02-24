package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

var port *int

var rootCmd = func() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "httpdump",
		Short: "Simple web server that dumps all requests it gets to the file system",
		Run: func(cmd *cobra.Command, args []string) {
			if port == nil {
				panic("port required")
			}

			fmt.Printf("Running httpdump on port %d\n", *port)
			err := http.ListenAndServe(fmt.Sprintf("0.0.0.0:%d", *port), &handler{})
			if err != nil {
				panic(err)
			}
		},
	}

	port = cmd.Flags().Int("port", 80, "Port to listen on")

	return cmd
}()

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

type handler struct {
}

func (handler) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	sb := strings.Builder{}

	fmt.Printf("%s %s", req.Method, req.URL.String())

	sb.WriteString("# Generated by httpdump - right now these requests aren't replayable because the URLs are prettified\n\n")

	sb.WriteString(req.Method)
	sb.WriteRune(' ')
	sb.WriteString(req.URL.String())
	sb.WriteString("\n")

	for key, value := range req.Header {
		sb.WriteString(key)
		sb.WriteString(": ")
		sb.WriteString(strings.Join(value, ", "))
		sb.WriteString("\n")
	}
	sb.WriteRune('\n')

	body, err := io.ReadAll(req.Body)
	if err != nil {
		panic(err)
	}

	sb.WriteString(string(body))
	err = req.Body.Close()
	if err != nil {
		panic(err)
	}

	timestamp := time.Now().UnixNano()
	err = os.WriteFile(fmt.Sprintf("./%d.http", timestamp), []byte(sb.String()), os.ModePerm)
	if err != nil {
		panic(err)
	}

	res.WriteHeader(http.StatusOK)
}
