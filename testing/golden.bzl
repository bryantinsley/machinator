def golden_test(name, binary, golden_file, args = [], data = []):
    """A macro for golden file tests.

    Args:
        name: Name of the test target.
        binary: The binary to run.
        golden_file: The golden file to compare against.
        args: Extra arguments to pass to the binary.
        data: Additional data dependencies.
    """

    # The runner script is in //testing:golden_test.sh
    # We use sh_test for the comparison
    native.sh_test(
        name = name,
        srcs = ["//testing:golden_test.sh"],
        data = [binary, golden_file] + data,
        args = [
            "$(rootpath {})".format(binary),
            "$(rootpath {})".format(golden_file),
        ] + args,
        env = {
            "BAZEL_TARGET": native.package_name() + ":" + name,
        },
    )

    # We use sh_binary for updating
    native.sh_binary(
        name = name + ".update",
        srcs = ["//testing:golden_test.sh"],
        data = [binary, golden_file] + data,
        args = [
            "$(rootpath {})".format(binary),
            "$(rootpath {})".format(golden_file),
        ] + args + ["--update"],
        env = {
            "GOLDEN_SOURCE_PATH": "$(rootpath {})".format(golden_file),
        },
    )
