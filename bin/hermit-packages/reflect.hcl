description = "reflect"
test        = "reflect --version"
binaries    = ["reflect"]

version "0.0.22" {
  source = "https://github.com/juliaogris/reflect/releases/download/v${version}/reflect_${version}_${os}_${arch}.tar.gz"
}
