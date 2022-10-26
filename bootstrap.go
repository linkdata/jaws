package jaws

const BootstrapDefaultVersion = "5.2.2"
const BootstrapDefaultCDN = "https://cdn.jsdelivr.net/npm/bootstrap"

type BootstrapConfig struct {
	Version      string
	CDN          string
	bootstrapCSS string
	bootstrapJS  string
}

var bootstrapConfig *BootstrapConfig

// UseBootstrap allows customizing the Bootstrap configuration.
// If not called explicitly, the first JaWS instance will call it.
// This is not thread-safe, so must be called before using a JaWS instance.
// Passing nil sets the default configuration.
func UseBootstrap(bc *BootstrapConfig) {
	if bc == nil {
		bc = &BootstrapConfig{}
	}
	bc.init()
	bootstrapConfig = bc
}

func (bc *BootstrapConfig) init() {
	if bc.Version == "" {
		bc.Version = BootstrapDefaultVersion
	}
	if bc.CDN == "" {
		bc.CDN = BootstrapDefaultCDN
	}
	bc.bootstrapCSS = bc.CDN + "@" + bc.Version + "/dist/css/bootstrap.min.css"
	bc.bootstrapJS = bc.CDN + "@" + bc.Version + "/dist/js/bootstrap.bundle.min.js"
}
