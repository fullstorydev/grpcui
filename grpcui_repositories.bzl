"""Bazel macro to run a binary copy of gRPC UI."""


# sample link (for `_bindist("1.8.6", "osx", "arm64")`):
# https://github.com/fullstorydev/grpcui/releases/download/v1.8.6/grpcui_1.8.6_osx_arm64.tar.gz
def _bindist(version, os, arch):
    return "https://github.com/fullstorydev/grpcui/releases/download/v%s/grpcui_%s_%s_%s.tar.gz" % (
        version,
        version,
        os,
        arch
    )

# sample bundle (for `_bindist_bundle("1.8.6", "linux", archs = ["arm64", "s390x", "x86_32", "x86_64"])`):
# "linux": {
#     "arm64": _bindist("1.8.6", "linux", "arm64"),
#     "s390x": _bindist("1.8.6", "linux", "s390x"),
#     "x86_32": _bindist("1.8.6", "linux", "x86_32"),
#     "x86_64": _bindist("1.8.6", "linux", "x86_64"),
# },
def _bindist_bundle(version, os, archs = []):
    return dict([
        (arch, _bindist(version, os, arch))
        for arch in archs
    ])

# sample version: (for `_bindist_bundle("1.8.6", bundles = {"osx": ["arm64"], "linux": ["arm64"]})`)
# "1.8.6": {
#     "linux": {
#         "arm64": _bindist("1.8.6", "linux", "arm64"),
#     },
#     "darwin": {
#         "arm64": _bindist("1.8.6", "osx", "arm64"),
#     },
# },
def _bindist_version(version, bundles = {}):
    return dict([
        (os, _bindist_bundle(version, os, archs))
        for os, archs in bundles.items()
    ])


# version checkums (static)
_grpcui_version_checksums = {
    "1.3.0_linux_x86_32": "0d570326b95305414aaf841a8793f23c2236930b4db7c6121d26bfd2a75da6f2",
    "1.3.0_linux_arm64": "1c55cca265a29bc9825c3c4a10e882bbfa9c56795b0674be7a8fce8d3ce2f6ed",
    "1.3.0_osx_x86_64": "2525239a1e805e1c3fe6f1837ed79fbb4cb5daa1f4da96e0f84b7d0c57e801eb",
    "1.3.0_osx_arm64": "6396a78776c06eff5935eb5763e7ffb6760e49a5f2e872113d4fadde5cc3cb35",
    "1.3.0_linux_x86_64": "9a7ebe31b89d585a80971f3795b3a8ada9345499c5f987a5a24d02368c314fae",
}

# version configs (static)
_grpcui_version_configs = {
    "1.3.0": _bindist_version(
        version = "1.3.0",
        bundles = {
            "linux": ["arm64", "x86_32", "x86_64"],
            "osx": ["arm64", "x86_64"],
        },
    ),
}

_grpcui_latest_version = "1.3.0"

def _get_platform(ctx):
    res = ctx.execute(["uname", "-p"])
    arch = "amd64"
    if res.return_code == 0:
        uname = res.stdout.strip()
        if uname == "arm":
            arch = "arm64"
        elif uname == "aarch64":
            arch = "aarch64"

    if ctx.os.name == "linux":
        return ("linux", arch)
    elif ctx.os.name == "mac os x":
        if arch == "arm64" or arch == "aarch64":
            return ("osx", "arm64")
        return ("osx", "x86_64")
    else:
        fail("Unsupported operating system: " + ctx.os.name)

def _grpcui_bindist_repository_impl(ctx):
    platform = _get_platform(ctx)
    version = ctx.attr.version

    # resolve dist
    config = _grpcui_version_configs[version]
    link = config[platform[0]][platform[1]]
    sha = _grpcui_version_checksums["%s_%s_%s" % (version, platform[0], platform[1])]

    urls = [link]
    ctx.download_and_extract(
        url = urls,
        sha256 = sha,
    )

    ctx.file("BUILD", """exports_files(glob(["**/*"]))""")
    ctx.file("WORKSPACE", "workspace(name = \"{name}\")".format(name = ctx.name))


grpcui_bindist_repository = repository_rule(
    attrs = {
        "version": attr.string(
            mandatory = True,
            default = _grpcui_latest_version,
        ),
    },
    implementation = _grpcui_bindist_repository_impl,
)
