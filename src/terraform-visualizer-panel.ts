'use strict';
import * as path from 'path';
import * as vscode from 'vscode';
import { PreviewDocumentContentProvider, SourceType } from './preview-document-content-provider';
import { PreviewKind } from './core';
import { outputFileSync } from 'fs-extra';
const hcl = require('./hcl-hil.js');

export default class TerraformVisualizerPanel extends PreviewDocumentContentProvider {
    /**
     * Tracke the current panel.  Only allow a single panel to exist at a time.
     */
    public static currentPanel: TerraformVisualizerPanel | undefined;
    private _panel: vscode.WebviewPanel | undefined;
    private _column: vscode.ViewColumn;
    private readonly _extensionPath: string;
    private readonly _workspaceRoot: string | undefined;
    private readonly _onDiskPath: vscode.Uri;
    private readonly _localSourceUri: any;
    private _disposables: vscode.Disposable[] = [];

    constructor(extensionPath: string, column: vscode.ViewColumn) {
        super();

        this._workspaceRoot = extensionPath;
        this._extensionPath = extensionPath;
        this._column = column;
        this._onDiskPath = vscode.Uri.file(path.join(this._extensionPath, 'web'));
        this._localSourceUri = this._onDiskPath.with({ scheme: 'vscode-resource' });



        if (vscode.workspace.workspaceFolders != undefined) {
            this._workspaceRoot = vscode.workspace.workspaceFolders[0].uri.fsPath
        }
    }

    public dispose() {
        TerraformVisualizerPanel.currentPanel = undefined;

        // Clean up our resources
        if (this._panel)
            this._panel.dispose();

        while (this._disposables.length) {
            const x = this._disposables.pop();
            if (x) {
                x.dispose();
            }
        }
    }

    protected getPreviewKind(): PreviewKind {
        return "directory";
    }


    // Ext returns the Terraform configuration extension of the given
    // path, or a blank string if it is an invalid function.
    protected ext(path: string) {
        if (path.endsWith('.tf')) {
            return '.tf'
        } else if (path.endsWith('.tf.json')) {
            return '.tf.json'
        }

        return ''
    }

    public drawDiagram(): string {

        if (vscode.workspace.workspaceFolders == undefined) {
            return "";
        }


        // If we already have a panel, show it.
        // Otherwise create a new panel
        if (TerraformVisualizerPanel.currentPanel && TerraformVisualizerPanel.currentPanel._panel) {

            TerraformVisualizerPanel.currentPanel._panel.reveal(this._column);

        } else {
            var htmlContent = "";
            try {
                htmlContent = this._getHtml();
            } catch (e) {
                return "";
            }
            TerraformVisualizerPanel.currentPanel = this;
            this._panel = vscode.window.createWebviewPanel("AutoDiagram", "Terraform Visualizer", this._column, {
                // Enable javascript in the webview
                enableScripts: true,
                retainContextWhenHidden: true,

                // And restric the webview to only loading content from our extension's `media` directory.
                localResourceRoots: [
                    vscode.Uri.file(path.join(this._extensionPath, 'web'))
                ]
            });

            this._panel.webview.html = htmlContent;

            // Update the content based on view changes
            this._panel.onDidChangeViewState(e => {
                if (this._panel)
                    if (this._panel.viewColumn)
                        this._column = this._panel.viewColumn
            }, null, this._disposables);

            // Listen for when the panel is disposed
            // This happens when the user closes the panel or when the panel is closed programatically
            this._panel.onDidDispose(() => this.dispose(), null, this._disposables);

            // Handle messages from the webview
            this._panel.webview.onDidReceiveMessage(message => {
                switch (message.command) {
                    case 'alert':
                        vscode.window.showErrorMessage(message.text);
                        return;
                }
            }, null, this._disposables);
        }



        if (this._panel)
            return this._panel.webview.html
        return ""
    }

    private _getHtml() {

        const nonce = this.getNonce();
        var data;
        try {
            data = hcl.dirToCytoscape(this._workspaceRoot);
        } catch (e) {
            console.log(e);
            vscode.window.showErrorMessage(e + '');
            throw new Error(e);
        }

        console.log("cytoscape_data:", data);
        outputFileSync(this._onDiskPath.fsPath + "/.tv/data.json", data);


        return `
                                <!DOCTYPE html>
                                <meta charset="utf-8">  
                                <html>
                                <head>
                                    <!--
                                    Use a content security policy to only allow loading images from https or from our extension directory,
                                    and only allow scripts that have a specific nonce.
                                    -->
                                    
                                    <base>
                                    <title>Ixia Terraform Visualizer</title>
                                    <script nonce="${nonce}" src="${this._localSourceUri}/js/nprogress.js"></script>
                                    <link href="${this._localSourceUri}/css/nprogress.css" rel="stylesheet" type="text/css" />           
                                    <link href="${this._localSourceUri}/css/jquery.qtip.css" rel="stylesheet" type="text/css" />
                                    <link href="${this._localSourceUri}/css/cytoscape.js-panzoom.css" rel="stylesheet" type="text/css" />
                                    <link href="${this._localSourceUri}/css/font-awesome-4.7.0/css/font-awesome.css" rel="stylesheet" type="text/css" />                              
                                    <link href="${this._localSourceUri}/css/cytoscape.js-navigator.css" rel="stylesheet" type="text/css" />
                                    <link href="${this._localSourceUri}/css/akkordion.css" rel="stylesheet" type="text/css" />       
                                    <link href="${this._localSourceUri}/css/cloudmap.css" rel="stylesheet" type="text/css" />
                                
                                    <link rel="shortcut icon" type="image/x-icon" href="${this._localSourceUri}/favicon.ico" />
                                </head>
                                
                                <body>
                                <div id="menu">
                                <b id="hide" class="fa fa-eye-slash fa-lg tooltip"><span class="tooltiptext">Delete</span></b>
                                <b id="showAll" class="fa fa-eye fa-lg tooltip"><span class="tooltiptext">Undelete all</span></b>
                                &nbsp;  
                                <b id="highlightNeighbors" class="fa fa-share-alt fa-lg tooltip"><span class="tooltiptext">Highlight neighbors</span></b>
                                <b id="removeHighlights" class="fa fa-share-alt-square fa-lg tooltip"><span class="tooltiptext">Remove highlight of neighbors</span></b>
                                &nbsp;
                                <b id="collapseAll" class="fa fa-compress fa-lg tooltip"><span class="tooltiptext">Collapse all</span></b> 
                                <b id="expandAll" class="fa fa-expand fa-lg tooltip"><span class="tooltiptext">Expand all</span></b>
                                &nbsp;
                                <b id="collapseRecursively" class="fa fa-minus fa-lg tooltip"><span class="tooltiptext">Collapse selected</span></b>
                                <b id="expandRecursively" class="fa fa-plus fa-lg tooltip"><span class="tooltiptext">Expand selected</span></b>
                                &nbsp;
                                <b id="randomizeLayout" class="fa fa-gavel fa-lg tooltip"><span class="tooltiptext">Redraw with randomized layout</span></b>
                                &nbsp;
                                <b id="saveImage" class="fa fa-camera fa-lg tooltip"><span class="tooltiptext">Save as image</span></b>
                                &nbsp; 
                                <b id="exportLayout" class="fa fa-download fa-lg tooltip"><span class="tooltiptext">Export</span></b>
                                <label for="fileUpload"><b id="importLayout" style="display: inline-block;" class="fa fa-upload fa-lg tooltip"><span class="tooltiptext">Import</span></b></label>
                                <input type="file" id="fileUpload" style="display: none;" onchange="importLayout()">
                        
                                <div id="nodeInfo">
                                        <div id="nodeLocation">&nbsp;</div>
                                        <div class="akkordion" data-akkordion-single="true" data-akkordion-speed="200" id="nodeDescriptionHolder">
                                            <div class="akkordion-title nodeHeader"><span id="nodeName">Welcome to Geppetto</span></div>
                                            <div class="akkordion-content frame akkordion-active" id="nodeDescription">
                                                <div class="akkordion" data-akkordion-single="false" data-akkordion-speed="200" id="nodeDescriptionHolder">
                                                    <div class="akkordion-title">Summary</div>
                                                    <div class="akkordion-content akkordion-active" id="Summary">
                                                        Once the graph has loaded, click around on icons and expand these windows to learn more about the nodes.
                                                    </div>
                                                    <div class="akkordion-title">Details</div>
                                                    <div class="akkordion-content" id="Details">None</div>
                                                    <div class="akkordion-title">Neighbors</div>
                                                    <div class="akkordion-content" id="Neighbors"></div>
                                                    <div class="akkordion-title">Siblings</div>
                                                    <div class="akkordion-content" id="Siblings"></div>
                                                    <div class="akkordion-title">Children</div>
                                                    <div class="akkordion-content" id="Children"></div>
                                                </div>
                                            </div>
                                        </div>
                                </div>
                            </div>
                        
                            <div id="cy"></div>
                            <script nonce="${nonce}" src="${this._localSourceUri}/js/cytoscape.min.js"></script>
                            <script nonce="${nonce}" src="${this._localSourceUri}/js/jquery.min.js"></script>
                            <script nonce="${nonce}" src="${this._localSourceUri}/js/jquery.qtip.js"></script>
                            <script nonce="${nonce}" src="${this._localSourceUri}/js/cytoscape-cose-bilkent.js"></script>
                            <script nonce="${nonce}" src="${this._localSourceUri}/js/cytoscape-grid-guide.js"></script>
                            <script nonce="${nonce}" src="${this._localSourceUri}/js/cytoscape-qtip.js"></script>                       
                            <script nonce="${nonce}" src="${this._localSourceUri}/js/cytoscape-panzoom.js"></script>
                            <script nonce="${nonce}" src="${this._localSourceUri}/js/cytoscape-undo-redo.js"></script>
                            <script nonce="${nonce}" src="${this._localSourceUri}/js/cytoscape-view-utilities.js"></script>
                            <script nonce="${nonce}" src="${this._localSourceUri}/js/cytoscape-expand-collapse.js"></script>
                            <script nonce="${nonce}" src="${this._localSourceUri}/js/cytoscape-navigator.js"></script>
                            <script nonce="${nonce}" src="${this._localSourceUri}/js/cytoscape-autopan-on-drag.js"></script>                        
                            <script nonce="${nonce}" src="${this._localSourceUri}/js/FileSaver.min.js"></script>
                            <script nonce="${nonce}" src="${this._localSourceUri}/js/circular-json.js"></script>
                            <script nonce="${nonce}" src="${this._localSourceUri}/js/mousetrap.min.js"></script>
                            <script nonce="${nonce}" src="${this._localSourceUri}/js/akkordion.min.js"></script>
                        
                            <script nonce="${nonce}" src="${this._localSourceUri}/js/nodeInfo.js"></script>
                            <script nonce="${nonce}" src="${this._localSourceUri}/js/cloudmap.js"></script>                            
                                    <script type="text/javascript">

                                               function setError(e) {
                                                   var mapEl = document.getElementById("map");
                                                   var errHtml = "<h1>An error occurred rendering diagram</h1>";
                                                   errHtml += "<pre>" + e.stack + "</pre>";
                                                   mapEl.innerHTML = errHtml;
                                               }
                                   
                                               try {
                                                   $(window).on('load', function(){
                                                       NProgress.set(0.1);
                                                         
                                                       akkordion(".akkordion", {});
                                                       
                                                       $.when(
                                                           $.getJSON("${this._localSourceUri}/.tv/data.json"),
                                                           $.getJSON("${this._localSourceUri}/style.json")
                                                       ).done(function(datafile, stylefile) {
                                                           var sFile = JSON.parse(JSON.stringify(stylefile[0]).replace("\${localSourceUri}", "${this._localSourceUri}"))
                                                           NProgress.set(0.5);

                                                           loadCytoscape({
                                                               wheelSensitivity: 0.1,
                                                               container: document.getElementById('cy'),
                                                               elements: datafile[0],
                                                               layout: {
                                                                   name: 'cose-bilkent',
                                                                   nodeDimensionsIncludeLabels: true,
                                                                   tilingPaddingVertical: 10,
                                                                   tilingPaddingHorizontal: 100
                                                               },
                                                               style: sFile
                                                           });
                                                       });
                                                   }); // Page loaded
                                               } catch (e) {
                                                   setError(e);
                                               }
                                           </script>
                                    </body>
                                </html>`
    }
    private getNonce() {
        let text = "";
        const possible = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789";
        for (let i = 0; i < 32; i++) {
            text += possible.charAt(Math.floor(Math.random() * possible.length));
        }
        return text;
    }
}