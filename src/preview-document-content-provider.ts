'use strict';
import * as fs from 'fs';
import * as vscode from 'vscode';
import * as path from 'path';
import { makePreviewUri, PreviewKind } from './core';

export enum SourceType {
    SCRIPT,
    STYLE
}

export abstract class PreviewDocumentContentProvider implements vscode.TextDocumentContentProvider {
    protected _onDidChange = new vscode.EventEmitter<vscode.Uri>();
    protected _projections = new Map<string, string>();
    protected _subscriptions: vscode.Disposable;

    constructor() {
        this._subscriptions = vscode.Disposable.from(
            vscode.workspace.onDidOpenTextDocument(this.onDocumentOpened.bind(this))
        );
    }

    dispose() {
        this._projections.clear();
        this._subscriptions.dispose();
        this._onDidChange.dispose();
    }

    protected abstract getPreviewKind(): PreviewKind;

    onDocumentOpened(e: vscode.TextDocument): void {
        //console.log(`Document opened ${e.uri}`);
        const uri = makePreviewUri(this.getPreviewKind(), e);
        this._onDidChange.fire(uri);
    }

    public triggerVirtualDocumentChange(uri: vscode.Uri): void {
        this._onDidChange.fire(uri);
    }

    private resolveDocument(uri: vscode.Uri): vscode.TextDocument | null {
        const matches = vscode.window.visibleTextEditors.filter(ed => {
            return makePreviewUri(this.getPreviewKind(), ed.document).toString() == uri.toString();
        });
        if (matches.length == 1) {
            return matches[0].document;
        } else {
            return null;
        }
    }

    public clearPreviewProjection(uri: vscode.Uri): void {
        this._projections.delete(uri.toString());
    }

    public setPreviewProjection(uri: vscode.Uri, projection: string): void {
        this._projections.set(uri.toString(), projection);
    }

    private generateDocumentContent(uri: vscode.Uri): string {
        const doc = this.resolveDocument(uri);
        if (doc) {
            const content = this.drawDiagram();
            const debugSettings = vscode.workspace.getConfiguration("terraform.visualize.debug");
            if (debugSettings.has("tbd")) {
                const doSomething = debugSettings.get("tbd");
                if (doSomething) {
                    try {
                        // do something
                    }
                    catch (e) {
                        // vscode.window.showErrorMessage(`Error : ${e.message}`);
                    }
                }
            }
            return content;
        } else {
            return this.errorSnippet(`<h1>Error preparing preview</h1><p>Cannot resolve document for virtual document URI: ${uri.toString()}</p>`);
        }
    }

    public provideTextDocumentContent(uri: vscode.Uri): string {
        const content = this.generateDocumentContent(uri);
        const sUri = uri.toString();
        return content;
    }

    private errorSnippet(error: string): string {
        return `
            <body>
                ${error}
            </body>`;
    }

    /**
     * Expose an event to signal changes of _virtual_ documents
     * to the editor
     */
    get onDidChange(): vscode.Event<vscode.Uri> {
        return this._onDidChange.event;
    }

    protected createStaticFileUri(file: string, dir: string): string {
        return vscode.Uri.file(
            path.join(
                __dirname,
                "..",
                "..",
                dir,
                file
            )
        ).toString();
    }
    protected readStaticFile(file: string, dir: string): Thenable<string> {
        return vscode.workspace.openTextDocument(this.createStaticFileUri(file, dir)).then(doc => {
            return doc.getText();
        });
    }

    protected createDevToolsLink(left: number, top: number): string {
        return `<div style="position: absolute; left: ${left}; top: ${top}; z-index: 100; padding: 5px; background: white"><a href="command:_webview.openDevTools">Open DevTools</a></div>`;
    }

    protected createLocalSource(file: string, type: SourceType, dir: string) {
        const source_path = this.createStaticFileUri(file, dir);
        switch (type) {
            case SourceType.SCRIPT:
                return `<script src="${source_path}" type="text/javascript"></script>`;
            case SourceType.STYLE:
                return `<link href="${source_path}" rel="stylesheet" />`;
        }
    }

    protected cleanText(text: string): string {
        const scrubRegexes = [
            { regex: /\\(?!\\|\/|\})/g, replace: "\\\\" },        //Existing backslashes
            { regex: /(<\!\[CDATA\[[\s\S]*?]]>)/g, replace: "" }, //CDATA blocks in XML
            { regex: /`/g, replace: "\\`" },                      //Backticks
            { regex: /\${/g, replace: "\\${" }                    //Start of ES6 template string placeholder
        ];
        for (const r of scrubRegexes) {
            text = text.replace(r.regex, r.replace);
        }
        return text;
    }

    public abstract drawDiagram(): string;
}