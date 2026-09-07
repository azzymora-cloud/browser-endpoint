#!/usr/bin/env python3
"""Generate a wixl-compatible WiX source from a staged install tree."""
from __future__ import annotations

import hashlib
import os
import sys
import uuid
from pathlib import Path

UPGRADE_CODE = "9F2C1A70-4B8E-4E31-9C6D-2A7B1E8F3D04"
VERSION = "1.1.0"
SKIP_DIR_NAMES = {".git", "dist", "__pycache__", "agent"}


def file_id(rel: str) -> str:
    digest = hashlib.sha1(rel.encode("utf-8")).hexdigest()[:10]
    return "f" + digest


def dir_id(rel: str) -> str:
    if not rel or rel == ".":
        return "INSTALLDIR"
    digest = hashlib.sha1(rel.encode("utf-8")).hexdigest()[:10]
    return "d" + digest


def guid_for(rel: str) -> str:
    return str(uuid.uuid5(uuid.UUID(UPGRADE_CODE), rel)).upper()


def collect(stage: Path) -> list[Path]:
    files: list[Path] = []
    for path in sorted(stage.rglob("*")):
        if not path.is_file():
            continue
        rel_parts = path.relative_to(stage).parts
        if any(p in SKIP_DIR_NAMES for p in rel_parts):
            continue
        if path.name.endswith(".msi"):
            continue
        files.append(path)
    return files


def emit(stage: Path) -> str:
    files = collect(stage)
    dirs = {""}
    for path in files:
        rel = path.relative_to(stage)
        parent = rel.parent
        while True:
            key = "" if parent == Path(".") else parent.as_posix()
            dirs.add(key)
            if parent == Path(".") or parent == Path(""):
                break
            parent = parent.parent

    dir_xml = []
    # Children grouped by parent posix path
    children: dict[str, list[str]] = {}
    for d in dirs:
        parent = "" if d in ("", ".") else str(Path(d).parent.as_posix() if Path(d).parent != Path(".") else "")
        if d in ("", "."):
            continue
        children.setdefault(parent, []).append(d)

    def render_dir(key: str, indent: int) -> list[str]:
        lines: list[str] = []
        pad = "  " * indent
        name = Path(key).name
        lines.append(f'{pad}<Directory Id="{dir_id(key)}" Name="{name}">')
        for child in sorted(children.get(key, [])):
            lines.extend(render_dir(child, indent + 1))
        # files in this dir
        for path in files:
            rel = path.relative_to(stage)
            parent = "" if rel.parent == Path(".") else rel.parent.as_posix()
            if parent != key:
                continue
            rid = file_id(rel.as_posix())
            src = path.as_posix()
            lines.append(f'{pad}  <Component Id="c{rid[1:]}" Guid="{guid_for(rel.as_posix())}">')
            lines.append(
                f'{pad}    <File Id="{rid}" Source="{src}" KeyPath="yes" />'
            )
            if rel.name == "thinkcentre-agent.exe":
                lines.append(
                    f'{pad}    <Shortcut Id="ManageShortcut" Directory="ProgramMenuDir" '
                    f'Name="ThinkCentre Endpoint" Description="Open the management console" '
                    f'Target="[INSTALLDIR]ThinkCentre-Endpoint.url" WorkingDirectory="INSTALLDIR" />'
                )
            lines.append(f"{pad}  </Component>")
        lines.append(f"{pad}</Directory>")
        return lines

    root_files = []
    pad = "          "
    for path in files:
        rel = path.relative_to(stage)
        if rel.parent != Path("."):
            continue
        rid = file_id(rel.as_posix())
        root_files.append(f'{pad}<Component Id="c{rid[1:]}" Guid="{guid_for(rel.as_posix())}">')
        root_files.append(f'{pad}  <File Id="{rid}" Source="{path.as_posix()}" KeyPath="yes" />')
        if rel.name == "thinkcentre-agent.exe":
            root_files.append(
                f'{pad}  <Shortcut Id="ManageShortcut" Directory="ProgramMenuDir" '
                f'Name="ThinkCentre Endpoint" Description="Open the management console" '
                f'Target="[INSTALLDIR]ThinkCentre-Endpoint.url" WorkingDirectory="INSTALLDIR" />'
            )
            root_files.append(
                f'{pad}  <ServiceInstall Id="AgentService" Type="ownProcess" '
                f'Name="ThinkCentreEndpoint" DisplayName="ThinkCentre Endpoint Agent" '
                f'Description="Manages the ThinkCentre browser remote stack, Wake-on-LAN, and reboot." '
                f'Start="auto" ErrorControl="normal" Account="LocalSystem" />'
            )
            root_files.append(
                f'{pad}  <ServiceControl Id="AgentServiceControl" Name="ThinkCentreEndpoint" '
                f'Start="install" Stop="both" Remove="uninstall" Wait="yes" />'
            )
        root_files.append(f"{pad}</Component>")

    nested = []
    for child in sorted(children.get("", [])):
        nested.extend(render_dir(child, 5))

    component_refs = []
    for path in files:
        rid = file_id(path.relative_to(stage).as_posix())
        component_refs.append(f'        <ComponentRef Id="c{rid[1:]}" />')

    agent_id = file_id("thinkcentre-agent.exe")

    return f'''<?xml version="1.0" encoding="utf-8"?>
<Wix xmlns="http://schemas.microsoft.com/wix/2006/wi">
  <Product Id="*" Name="ThinkCentre Endpoint" Language="1033" Version="{VERSION}"
           Manufacturer="ThinkCentre Endpoint" UpgradeCode="{UPGRADE_CODE}">
    <Package Description="ThinkCentre browser remote agent, stack files, Wake-on-LAN, and reboot."
             Comments="Per-machine install" InstallerVersion="300" Compressed="yes"
             InstallScope="perMachine" />
    <Media Id="1" Cabinet="media1.cab" EmbedCab="yes" />
    <Directory Id="TARGETDIR" Name="SourceDir">
      <Directory Id="ProgramFiles64Folder">
        <Directory Id="INSTALLDIR" Name="ThinkCentre Endpoint">
{os.linesep.join(root_files)}
{os.linesep.join(nested)}
        </Directory>
      </Directory>
      <Directory Id="ProgramMenuFolder">
        <Directory Id="ProgramMenuDir" Name="ThinkCentre Endpoint" />
      </Directory>
    </Directory>
    <Feature Id="MainFeature" Title="ThinkCentre Endpoint" Level="1">
{os.linesep.join(component_refs)}
    </Feature>
    <CustomAction Id="EnableWOL" FileKey="{agent_id}" ExeCommand="enable-wol"
                  Execute="deferred" Impersonate="no" Return="ignore" />
    <InstallExecuteSequence>
      <Custom Action="EnableWOL" After="InstallServices">NOT Installed</Custom>
    </InstallExecuteSequence>
  </Product>
</Wix>
'''


def main() -> int:
    if len(sys.argv) != 3:
        print("usage: generate-wxs.py STAGE_DIR OUT.wxs", file=sys.stderr)
        return 2
    stage = Path(sys.argv[1]).resolve()
    out = Path(sys.argv[2])
    out.write_text(emit(stage), encoding="utf-8")
    print(f"wrote {out} ({len(collect(stage))} files)")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
