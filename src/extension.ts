'use strict';
import * as path from 'path';
import * as vscode from 'vscode';
import TerraformVisualizerPanel from './terraform-visualizer-panel';
import * as extension from './core';

// this method is called when your extension is activated
// your extension is activated the very first time the command is executed
export function activate(context: vscode.ExtensionContext) {
    const tfVisualizer = new TerraformVisualizerPanel(context.extensionPath, vscode.ViewColumn.Two);
    const tfRegistration = vscode.workspace.registerTextDocumentContentProvider(extension.TERRAFORM_URI_SCHEME, tfVisualizer);

    const mapPreviewCommand = vscode.commands.registerCommand(extension.TERRAFORM_COMMAND_ID, () => {


        tfVisualizer.drawDiagram(context.extensionPath);
    });

    context.subscriptions.push(mapPreviewCommand, tfRegistration);
}

// this method is called when your extension is deactivated
export function deactivate() {

}