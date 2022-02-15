sources = ["env:///bin/hermit-packages", "https://github.com/cashapp/hermit-packages.git"]
manage-git = false
env = {
  GOBIN : "${HERMIT_ENV}/out/bin",
  PATH : "${GOBIN}:${PATH}",
}
