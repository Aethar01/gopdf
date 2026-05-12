class Gopdf < Formula
  desc "MuPDF-backend PDF viewer written in Go with Lua configuration"
  homepage "https://github.com/Aethar01/gopdf"
  url "https://github.com/Aethar01/gopdf/archive/refs/tags/0.1.10.tar.gz"
  sha256 "REPLACE_AFTER_TAGGING_0.1.10"
  license "AGPL-3.0-only"
  head "https://github.com/Aethar01/gopdf.git", branch: "main"

  livecheck do
    url :stable
    regex(/^v?(\d+(?:\.\d+)+)$/i)
  end

  depends_on "go" => :build
  depends_on "pkg-config" => :build
  depends_on "mupdf"
  depends_on "sdl2"

  def install
    pkgconfig = buildpath/"pkgconfig"
    pkgconfig.mkpath
    (pkgconfig/"mupdf.pc").write <<~EOS
      prefix=#{Formula["mupdf"].opt_prefix}
      exec_prefix=${prefix}
      libdir=${exec_prefix}/lib
      includedir=${prefix}/include

      Name: mupdf
      Description: MuPDF PDF rendering library
      Version: #{Formula["mupdf"].version}
      Libs: -L${libdir} -lmupdf -lmupdf-third -lm
      Cflags: -I${includedir}
    EOS
    ENV.prepend_path "PKG_CONFIG_PATH", pkgconfig

    system "go", "build", *std_go_args
  end

  test do
    assert_match(/\A\d+\.\d+\.\d+\s*\z/, shell_output("#{bin}/gopdf -v"))
  end
end
