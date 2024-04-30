package app

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/topolvm/topolvm"
	lvmd "github.com/topolvm/topolvm/cmd/lvmd/app"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var config struct {
	csiSocket           string
	lvmdSocket          string
	metricsAddr         string
	secureMetricsServer bool
	zapOpts             zap.Options
	embedLvmd           bool
	lvmd                lvmd.Config
}

var rootCmd = &cobra.Command{
	Use:     "topolvm-node",
	Version: topolvm.Version,
	Short:   "TopoLVM CSI node",
	Long: `topolvm-node provides CSI node service.
It also works as a custom Kubernetes controller.

The node name where this program runs must be given by either
NODE_NAME environment variable or --nodename flag.`,

	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		return subMain(cmd.Context())
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

//nolint:lll
func init() {
	fs := rootCmd.Flags()
	fs.StringVar(&config.csiSocket, "csi-socket", topolvm.DefaultCSISocket, "UNIX domain socket filename for CSI")
	fs.StringVar(&config.lvmdSocket, "lvmd-socket", topolvm.DefaultLVMdSocket, "UNIX domain socket of lvmd service")
	fs.StringVar(&config.metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	fs.BoolVar(&config.secureMetricsServer, "secure-metrics-server", false, "Secures the metrics server")
	fs.String("nodename", "", "The resource name of the running node")
	fs.BoolVar(&config.embedLvmd, "embed-lvmd", false, "Runs LVMD locally by embedding it instead of calling it externally via gRPC")
	fs.StringVar(&cfgFilePath, "config", filepath.Join("/etc", "topolvm", "lvmd.yaml"), "config file")

	_ = viper.BindEnv("nodename", "NODE_NAME")
	_ = viper.BindPFlag("nodename", fs.Lookup("nodename"))

	goflags := flag.NewFlagSet("klog", flag.ExitOnError)
	klog.InitFlags(goflags)
	config.zapOpts.BindFlags(goflags)

	fs.AddGoFlagSet(goflags)
}
