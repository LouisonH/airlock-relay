// airlock is the local operator CLI for an airlockd server core. It connects
// only to the user-only Unix control socket; route targets and credentials are
// supplied through protected specification files instead of process arguments.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/LouisonH/airlock-relay/internal/control"
	"github.com/LouisonH/airlock-relay/internal/securefile"
)

const version = "dev"

type rootOptions struct {
	socket    string
	dataDir   string
	tokenFile string
	timeout   time.Duration
}

type proxySpec struct {
	URL string `json:"url"`
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printUsage(stderr)
		return 2
	}
	if args[0] == "version" || args[0] == "--version" || args[0] == "-version" {
		_, _ = fmt.Fprintf(stdout, "airlock %s\n", version)
		return 0
	}
	if args[0] == "token" {
		return runToken(args[1:], stdout, stderr)
	}

	flags := flag.NewFlagSet("airlock", flag.ContinueOnError)
	flags.SetOutput(stderr)
	options := rootOptions{}
	flags.StringVar(&options.socket, "socket", "", "absolute airlockd Unix control socket")
	flags.StringVar(&options.dataDir, "data-dir", "", "absolute airlockd server data directory")
	flags.StringVar(&options.tokenFile, "token-file", "", "absolute protected 0600 control token file")
	flags.DurationVar(&options.timeout, "timeout", 20*time.Second, "control operation timeout")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	command := flags.Args()
	if len(command) == 0 {
		printUsage(stderr)
		return 2
	}
	client, err := newClient(options)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "airlock: %v\n", err)
		return 2
	}
	return runCommand(client, options.timeout, command, stdout, stderr)
}

func runToken(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "generate" {
		_, _ = fmt.Fprintln(stderr, "usage: airlock token generate --output /absolute/path")
		return 2
	}
	flags := flag.NewFlagSet("airlock token generate", flag.ContinueOnError)
	flags.SetOutput(stderr)
	output := flags.String("output", "", "absolute output path for a new 0600 token file")
	if err := flags.Parse(args[1:]); err != nil || flags.NArg() != 0 || *output == "" {
		if err == nil {
			_, _ = fmt.Fprintln(stderr, "usage: airlock token generate --output /absolute/path")
		}
		return 2
	}
	if _, err := securefile.CreateToken(*output); err != nil {
		_, _ = fmt.Fprintf(stderr, "airlock: %v\n", err)
		return 1
	}
	_, _ = fmt.Fprintf(stdout, "created protected token file: %s\n", *output)
	return 0
}

func newClient(options rootOptions) (*control.Client, error) {
	if options.timeout < time.Second || options.timeout > 2*time.Minute {
		return nil, errors.New("timeout must be between 1s and 2m")
	}
	if options.tokenFile == "" || !filepath.IsAbs(options.tokenFile) {
		return nil, errors.New("--token-file must be an absolute protected token path")
	}
	if options.socket != "" && options.dataDir != "" {
		return nil, errors.New("use either --socket or --data-dir")
	}
	socket := options.socket
	if socket == "" {
		if options.dataDir == "" || !filepath.IsAbs(options.dataDir) {
			return nil, errors.New("--data-dir must be an absolute server data directory when --socket is not used")
		}
		socket = filepath.Join(options.dataDir, "control.sock")
	}
	if !filepath.IsAbs(socket) {
		return nil, errors.New("--socket must be an absolute Unix socket path")
	}
	token, err := securefile.ReadToken(options.tokenFile)
	if err != nil {
		return nil, fmt.Errorf("read control token: %w", err)
	}
	return control.NewClient(socket, token)
}

func runCommand(client *control.Client, timeout time.Duration, args []string, stdout, stderr io.Writer) int {
	if len(args) == 1 && args[0] == "status" {
		return execute(client, timeout, control.Request{Action: "status"}, stdout, stderr)
	}
	if len(args) < 2 {
		printUsage(stderr)
		return 2
	}
	switch args[0] {
	case "routes":
		return runRoutes(client, timeout, args[1:], stdout, stderr)
	case "ssh":
		return runSSH(client, timeout, args[1:], stdout, stderr)
	case "proxy":
		return runProxy(client, timeout, args[1:], stdout, stderr)
	default:
		printUsage(stderr)
		return 2
	}
}

func runRoutes(client *control.Client, timeout time.Duration, args []string, stdout, stderr io.Writer) int {
	if len(args) == 1 && args[0] == "list" {
		return execute(client, timeout, control.Request{Action: "list_routes"}, stdout, stderr)
	}
	if len(args) == 2 {
		switch args[0] {
		case "enable":
			return execute(client, timeout, control.Request{Action: "set_route_enabled", Alias: args[1], Enabled: true}, stdout, stderr)
		case "disable":
			return execute(client, timeout, control.Request{Action: "set_route_enabled", Alias: args[1], Enabled: false}, stdout, stderr)
		case "health":
			return execute(client, timeout, control.Request{Action: "test_route_health", Alias: args[1]}, stdout, stderr)
		case "delete":
			return execute(client, timeout, control.Request{Action: "delete_route", Alias: args[1]}, stdout, stderr)
		}
	}
	if len(args) == 2 && args[0] == "create" {
		return runRouteCreate(client, timeout, args[1], nil, stdout, stderr)
	}
	if len(args) >= 2 && args[0] == "create" {
		return runRouteCreate(client, timeout, args[1], args[2:], stdout, stderr)
	}
	if len(args) == 2 && args[0] == "stop-all" && args[1] == "--yes" {
		return execute(client, timeout, control.Request{Action: "stop_all"}, stdout, stderr)
	}
	_, _ = fmt.Fprintln(stderr, "usage: airlock routes list|enable|disable|delete|health <alias> | create <http|llm|ssh> --file /absolute/spec.json | stop-all --yes")
	return 2
}

func runRouteCreate(client *control.Client, timeout time.Duration, kind string, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("airlock routes create", flag.ContinueOnError)
	flags.SetOutput(stderr)
	file := flags.String("file", "", "absolute protected 0600 route spec JSON")
	allowAllConfirmed := flags.Bool("allow-all-confirmed", false, "acknowledge unrestricted upstream SSH exec")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 || *file == "" || !filepath.IsAbs(*file) {
		if err == nil {
			_, _ = fmt.Fprintln(stderr, "route creation requires --file /absolute/protected-spec.json")
		}
		return 2
	}
	switch kind {
	case "http", "llm":
		var spec control.CreateHTTPRoute
		if err := readSpec(*file, &spec); err != nil {
			return printError(stderr, err)
		}
		defer clearHTTPRoute(&spec)
		action := "create_http_route"
		if kind == "llm" {
			action = "create_llm_route"
		}
		return executeCreate(client, timeout, control.Request{Action: action, Create: &spec}, stdout, stderr)
	case "ssh":
		var spec control.CreateSSHRoute
		if err := readSpec(*file, &spec); err != nil {
			return printError(stderr, err)
		}
		defer clearSSHRoute(&spec)
		if spec.AllowAllCommands && !*allowAllConfirmed {
			return printError(stderr, errors.New("unrestricted SSH exec requires --allow-all-confirmed"))
		}
		return executeSSHCreate(client, timeout, &spec, stdout, stderr)
	default:
		_, _ = fmt.Fprintln(stderr, "route kind must be http, llm, or ssh")
		return 2
	}
}

func runSSH(client *control.Client, timeout time.Duration, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "probe" {
		_, _ = fmt.Fprintln(stderr, "usage: airlock ssh probe --address host:port [--egress Direct|Proxy|Auto]")
		return 2
	}
	flags := flag.NewFlagSet("airlock ssh probe", flag.ContinueOnError)
	flags.SetOutput(stderr)
	address := flags.String("address", "", "upstream SSH host:port")
	egress := flags.String("egress", "Direct", "upstream egress policy")
	if err := flags.Parse(args[1:]); err != nil || flags.NArg() != 0 || *address == "" {
		if err == nil {
			_, _ = fmt.Fprintln(stderr, "SSH probe requires --address host:port")
		}
		return 2
	}
	return execute(client, timeout, control.Request{Action: "probe_ssh_host_key", ProbeSSH: &control.ProbeSSHHostKey{Address: *address, Egress: *egress}}, stdout, stderr)
}

func runProxy(client *control.Client, timeout time.Duration, args []string, stdout, stderr io.Writer) int {
	if len(args) == 2 && args[0] == "clear" && args[1] == "--yes" {
		return execute(client, timeout, control.Request{Action: "clear_proxy"}, stdout, stderr)
	}
	if len(args) == 0 || args[0] != "set" {
		_, _ = fmt.Fprintln(stderr, "usage: airlock proxy set --file /absolute/proxy.json | clear --yes")
		return 2
	}
	flags := flag.NewFlagSet("airlock proxy set", flag.ContinueOnError)
	flags.SetOutput(stderr)
	file := flags.String("file", "", "absolute protected 0600 proxy JSON")
	if err := flags.Parse(args[1:]); err != nil || flags.NArg() != 0 || *file == "" || !filepath.IsAbs(*file) {
		if err == nil {
			_, _ = fmt.Fprintln(stderr, "proxy setup requires --file /absolute/protected-proxy.json")
		}
		return 2
	}
	var spec proxySpec
	if err := readSpec(*file, &spec); err != nil {
		return printError(stderr, err)
	}
	defer func() { spec.URL = "" }()
	return execute(client, timeout, control.Request{Action: "configure_proxy", ProxyURL: spec.URL}, stdout, stderr)
}

func executeSSHCreate(client *control.Client, timeout time.Duration, spec *control.CreateSSHRoute, stdout, stderr io.Writer) int {
	created, err := call(client, timeout, control.Request{Action: "create_ssh_route", CreateSSH: spec})
	if err != nil {
		return printError(stderr, err)
	}
	if created.Created == nil {
		return printError(stderr, errors.New("airlockd did not return the created SSH route"))
	}
	writeJSON(stdout, created.Created)
	return 0
}

func executeCreate(client *control.Client, timeout time.Duration, request control.Request, stdout, stderr io.Writer) int {
	response, err := call(client, timeout, request)
	if err != nil {
		return printError(stderr, err)
	}
	if response.Created == nil {
		return printError(stderr, errors.New("airlockd did not return the created route"))
	}
	writeJSON(stdout, response.Created)
	return 0
}

func execute(client *control.Client, timeout time.Duration, request control.Request, stdout, stderr io.Writer) int {
	response, err := call(client, timeout, request)
	if err != nil {
		return printError(stderr, err)
	}
	writeJSON(stdout, response)
	return 0
}

func call(client *control.Client, timeout time.Duration, request control.Request) (control.Response, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	response, err := client.Do(ctx, request)
	if err != nil {
		return control.Response{}, err
	}
	if !response.OK {
		return control.Response{}, errors.New(response.Error)
	}
	return response, nil
}

func readSpec(path string, destination any) error {
	contents, err := securefile.Read(path, 64<<10)
	if err != nil {
		return fmt.Errorf("read protected spec: %w", err)
	}
	defer clear(contents)
	decoder := json.NewDecoder(strings.NewReader(string(contents)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return errors.New("decode protected spec")
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return errors.New("protected spec has trailing data")
	}
	return nil
}

func clearHTTPRoute(route *control.CreateHTTPRoute) {
	if route == nil {
		return
	}
	route.BaseURL, route.Authorization, route.LocalAPIKey = "", "", ""
}

func clearSSHRoute(route *control.CreateSSHRoute) {
	if route == nil {
		return
	}
	route.Address, route.Username, route.Password, route.LocalPassword, route.ExpectedHostKey = "", "", "", "", ""
}

func writeJSON(writer io.Writer, value any) {
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(value)
}

func printError(writer io.Writer, err error) int {
	_, _ = fmt.Fprintf(writer, "airlock: %v\n", err)
	return 1
}

func printUsage(writer io.Writer) {
	_, _ = fmt.Fprintln(writer, `usage: airlock [--data-dir /absolute/path | --socket /absolute/control.sock] --token-file /absolute/control.token <command>

commands:
  status
  routes list|enable|disable|delete|health <alias>
  routes create <http|llm|ssh> --file /absolute/protected-spec.json
  routes stop-all --yes
  ssh probe --address host:port [--egress Direct|Proxy|Auto]
  proxy set --file /absolute/protected-proxy.json
  proxy clear --yes
  token generate --output /absolute/token-file`)
}
