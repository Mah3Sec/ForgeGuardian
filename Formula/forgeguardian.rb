class Forgeguardian < Formula
  desc "AI-powered supply chain security scanner for software packages"
  homepage "https://github.com/mah3sec/forgeguardian"
  version "0.1.0"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/mah3sec/forgeguardian/releases/download/v#{version}/forgeguardian_#{version}_darwin_arm64.tar.gz"
      sha256 "PLACEHOLDER_DARWIN_ARM64"
    else
      url "https://github.com/mah3sec/forgeguardian/releases/download/v#{version}/forgeguardian_#{version}_darwin_amd64.tar.gz"
      sha256 "PLACEHOLDER_DARWIN_AMD64"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/mah3sec/forgeguardian/releases/download/v#{version}/forgeguardian_#{version}_linux_arm64.tar.gz"
      sha256 "PLACEHOLDER_LINUX_ARM64"
    else
      url "https://github.com/mah3sec/forgeguardian/releases/download/v#{version}/forgeguardian_#{version}_linux_amd64.tar.gz"
      sha256 "PLACEHOLDER_LINUX_AMD64"
    end
  end

  def install
    bin.install "fgctl"
    bin.install "fg-agent" if File.exist?("fg-agent")
    bin.install "intel-agent" if File.exist?("intel-agent")
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/fgctl version")
  end
end
