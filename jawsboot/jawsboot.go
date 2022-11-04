package jawsboot

import "github.com/linkdata/jaws"

const DefaultBootstrapVersion = "5.2.2"
const DefaultBootstrapCDN = "https://cdn.jsdelivr.net/npm/bootstrap"

// SetupUsing adds URL's for the given Bootstrap version and CDN to the Jaws.ExtraJS and Jaws.ExtraCSS arrays.
func SetupUsing(jw *jaws.Jaws, version, cdn string) {
	jw.GenerateHeadHTML(
		cdn+"@"+version+"/dist/js/bootstrap.bundle.min.js",
		cdn+"@"+version+"/dist/css/bootstrap.min.css",
	)
}

// Setup calls SetupUsing with DefaultBootstrapVersion and DefaultBootstrapCDN.
func Setup(jw *jaws.Jaws) {
	SetupUsing(jw, DefaultBootstrapVersion, DefaultBootstrapCDN)
}
