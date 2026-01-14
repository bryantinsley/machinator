load("@gazelle//:def.bzl", "gazelle")

# gazelle:prefix github.com/bryantinsley/machinator
gazelle(name = "gazelle")

gazelle(
    name = "gazelle-update-repos",
    args = [
        "-from_file=backend/go.mod",
        "-to_macro=deps.bzl%go_dependencies",
        "-prune",
    ],
    command = "update-repos",
)
