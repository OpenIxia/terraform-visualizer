'use strict';
import * as vscode from 'vscode';

export const EPSG_REGEX = /^EPSG:\d+$/g;
export const TERRAFORM_URI_SCHEME = "directory";
export const TERRAFORM_COMMAND_ID = "terraform.visualize";

export type PreviewKind = "config" | "directory";

export function makePreviewUri(kind: PreviewKind, doc: vscode.TextDocument): vscode.Uri {
    return vscode.Uri.parse(`${kind}://terraform-visualize: ${doc.fileName}`);
}