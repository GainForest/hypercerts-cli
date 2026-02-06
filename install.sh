#!/bin/bash
set -e

echo "Installing hc..."
go install github.com/GainForest/hypercerts-cli/cmd/hc@v0.1.1

GOBIN="$(go env GOPATH)/bin"

echo ""
echo "Installation complete!"
echo ""

# Check if GOBIN is in PATH
if command -v hc &> /dev/null; then
    echo "You can now use 'hc' in your terminal."
else
    echo "The 'hc' binary was installed to: $GOBIN/hc"
    echo ""
    echo "Add Go's bin directory to your PATH by adding this to your shell config:"
    echo ""
    if [ -f "$HOME/.zshrc" ]; then
        echo "  echo 'export PATH=\"\$PATH:$GOBIN\"' >> ~/.zshrc && source ~/.zshrc"
    elif [ -f "$HOME/.bashrc" ]; then
        echo "  echo 'export PATH=\"\$PATH:$GOBIN\"' >> ~/.bashrc && source ~/.bashrc"
    else
        echo "  export PATH=\"\$PATH:$GOBIN\""
    fi
    echo ""
    echo "Or run it directly:"
    echo "  $GOBIN/hc --help"
fi

echo ""
echo "Quick start:"
echo "  hc account login -u yourhandle.bsky.social -p your-app-password"
echo "  hc activity create"
echo "  hc --help"
