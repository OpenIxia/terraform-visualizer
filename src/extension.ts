'use strict';
import * as path from 'path';
import * as vscode from 'vscode';
import OpenLayersDocumentContentProvider from './ol-document-content-provider';
import * as extension from './core';

// this method is called when your extension is activated
// your extension is activated the very first time the command is executed
export function activate(context: vscode.ExtensionContext) {
    const olProvider = new OpenLayersDocumentContentProvider();
    const olRegistration = vscode.workspace.registerTextDocumentContentProvider(extension.TERRAFORM_URI_SCHEME, olProvider);

    const mapPreviewCommand = vscode.commands.registerCommand(extension.TERRAFORM_COMMAND_ID, () => {
        //        const doc = vscode.window.activeTextEditor.document;
        //        const previewUri = extension.makePreviewUri("map", doc);
        //        olProvider.clearPreviewProjection(previewUri);
        //        olProvider.triggerVirtualDocumentChange(previewUri);
        //        vscode.commands.executeCommand('vscode.previewHtml', previewUri, vscode.ViewColumn.Two).then((success) => {
        //
        //        }, (reason) => {
        //            vscode.window.showErrorMessage(reason);
        //        });
        const panel = vscode.window.createWebviewPanel("AutoDiagram", "Terraform Visualizer", vscode.ViewColumn.Two, {
            // Enable javascript in the webview
            enableScripts: true,

            // And restric the webview to only loading content from our extension's `media` directory.
            localResourceRoots: [
                vscode.Uri.file(path.join(context.extensionPath, 'web'))
            ]
        });

        panel.webview.html = olProvider.drawDiagram(context.extensionPath);
    });

    context.subscriptions.push(mapPreviewCommand, olRegistration);
}

// this method is called when your extension is deactivated
export function deactivate() {

}