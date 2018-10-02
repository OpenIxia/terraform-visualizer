'use strict';
import * as path from 'path';
import * as vscode from 'vscode';
import { PreviewDocumentContentProvider, SourceType } from './preview-document-content-provider';
import { PreviewKind } from './core';
import { outputFileSync } from 'fs-extra';
const hcl = require('./hcl-hil.js');

export default class OpenLayersDocumentContentProvider extends PreviewDocumentContentProvider {
    constructor() {
        super();
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

    public drawDiagram(extensionPath: string) {

        if (vscode.workspace.workspaceFolders == undefined) {
            return "";
        }
        const workspaceRootUri = vscode.workspace.workspaceFolders[0].uri;
        const workspaceRoot = workspaceRootUri.fsPath

        const onDiskPath = vscode.Uri.file(path.join(extensionPath, 'web'));
        const localSourceUri = onDiskPath.with({ scheme: 'vscode-resource' });

        var data, error;
        [data, error] = hcl.dirToCytoscape(workspaceRoot);

        if (error) {
            console.log("ERROR:", error);
        } else {

            console.log("cytoscape_data:", data);
            outputFileSync(onDiskPath.fsPath + "/.tv/data.json", data);
        }

        return `
                                <!DOCTYPE html>
                                <meta charset="utf-8">  
                                <html>
                                <head>
                                    <base>
                                    <title>Ixia Terraform Visualizer</title>
                                    <script src="${localSourceUri}/js/nprogress.js"></script>
                                    <link href="${localSourceUri}/css/nprogress.css" rel="stylesheet" type="text/css" />           
                                    <link href="${localSourceUri}/css/jquery.qtip.css" rel="stylesheet" type="text/css" />
                                    <link href="${localSourceUri}/css/cytoscape.js-panzoom.css" rel="stylesheet" type="text/css" />
                                    <link href="${localSourceUri}/css/font-awesome-4.7.0/css/font-awesome.css" rel="stylesheet" type="text/css" />                              
                                    <link href="${localSourceUri}/css/cytoscape.js-navigator.css" rel="stylesheet" type="text/css" />
                                    <link href="${localSourceUri}/css/akkordion.css" rel="stylesheet" type="text/css" />       
                                    <link href="${localSourceUri}/css/cloudmap.css" rel="stylesheet" type="text/css" />
                                
                                    <link rel="shortcut icon" type="image/x-icon" href="${localSourceUri}/favicon.ico" />
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
                            <script src="${localSourceUri}/js/cytoscape.min.js"></script>
                            <script src="${localSourceUri}/js/jquery.min.js"></script>
                            <script src="${localSourceUri}/js/jquery.qtip.js"></script>
                            <script src="${localSourceUri}/js/cytoscape-cose-bilkent.js"></script>
                            <script src="${localSourceUri}/js/cytoscape-grid-guide.js"></script>
                            <script src="${localSourceUri}/js/cytoscape-qtip.js"></script>                       
                            <script src="${localSourceUri}/js/cytoscape-panzoom.js"></script>
                            <script src="${localSourceUri}/js/cytoscape-undo-redo.js"></script>
                            <script src="${localSourceUri}/js/cytoscape-view-utilities.js"></script>
                            <script src="${localSourceUri}/js/cytoscape-expand-collapse.js"></script>
                            <script src="${localSourceUri}/js/cytoscape-navigator.js"></script>
                            <script src="${localSourceUri}/js/cytoscape-autopan-on-drag.js"></script>                        
                            <script src="${localSourceUri}/js/FileSaver.min.js"></script>
                            <script src="${localSourceUri}/js/circular-json.js"></script>
                            <script src="${localSourceUri}/js/mousetrap.min.js"></script>
                            <script src="${localSourceUri}/js/akkordion.min.js"></script>
                        
                            <script src="${localSourceUri}/js/nodeInfo.js"></script>
                            <script src="${localSourceUri}/js/cloudmap.js"></script>                            
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
                                                           $.getJSON("${localSourceUri}/.tv/data.json"),
                                                           $.getJSON("${localSourceUri}/style.json")
                                                       ).done(function(datafile, stylefile) {
                                                           var sFile = JSON.parse(JSON.stringify(stylefile[0]).replace("\${localSourceUri}", "${localSourceUri}"))
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
}