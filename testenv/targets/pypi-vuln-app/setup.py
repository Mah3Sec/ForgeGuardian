from setuptools import setup

# FORGEGUARDIAN TEST FIXTURE: Python package with known-vulnerable dependencies
# and unsafe code patterns for scanner testing.
setup(
    name='forgeguardian-test-pypi-vuln',
    version='1.0.0',
    description='ForgeGuardian test fixture: vulnerable Python patterns',
    install_requires=[
        'PyYAML==5.3.1',       # CVE-2020-14343: yaml.load() arbitrary code exec
        'Pillow==8.3.1',        # CVE-2021-34552: buffer overflow
        'requests==2.25.0',     # CVE-2023-32681: credential leak via redirect
        'paramiko==2.7.1',      # CVE-2022-24302: race condition on key generation
    ],
    py_modules=['app'],
)
