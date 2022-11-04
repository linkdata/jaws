package jawsboot

import "github.com/linkdata/jaws"

const DefaultBootstrapVersion = "5.2.2"
const DefaultBootstrapCDN = "https://cdn.jsdelivr.net/npm/bootstrap"

// SetupUsing adds URL's for the given Bootstrap version and CDN along with `extra` and calls Jaws.GenerateHeadHTML.
func SetupUsing(jw *jaws.Jaws, version, cdn string, extra ...string) error {
	return jw.GenerateHeadHTML(append(extra,
		cdn+"@"+version+"/dist/js/bootstrap.bundle.min.js",
		cdn+"@"+version+"/dist/css/bootstrap.min.css")...)
}

// Setup calls SetupUsing with DefaultBootstrapVersion and DefaultBootstrapCDN and `extra`.
func Setup(jw *jaws.Jaws, extra ...string) error {
	return SetupUsing(jw, DefaultBootstrapVersion, DefaultBootstrapCDN, extra...)
}
