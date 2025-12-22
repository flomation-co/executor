# Package Templates

This directory contains template files for building RPM and DEB packages.

## Template Syntax

Files in this directory use `{{PLACEHOLDER}}` syntax for variable substitution.
These placeholders are replaced during the build process by the `scripts/inject-metadata.sh` script.

## Note for Linters

Shell linters (like ShellCheck) will report errors on these template files because
`{{PLACEHOLDER}}` is not valid shell syntax. This is expected and can be safely ignored.

The templates are processed before being used, and the generated files in the project
root will have valid shell syntax.

## Directory Structure

- `apt/debian/` - Debian package templates
- `yum/template.spec` - RPM package template
