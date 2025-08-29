// Global state
let websocket = null;
let currentTab = 'agent-tab';

// Initialize the application
document.addEventListener('DOMContentLoaded', function() {
    initializeWebSocket();
    loadState();
    
    // Initialize mermaid
    mermaid.initialize({ 
        startOnLoad: true,
        theme: 'default',
        themeVariables: {
            primaryColor: '#007bff',
            primaryTextColor: '#333',
            primaryBorderColor: '#dee2e6',
            lineColor: '#666'
        }
    });
});

// WebSocket Connection
function initializeWebSocket() {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${window.location.host}/ws`;
    
    try {
        websocket = new WebSocket(wsUrl);
        
        websocket.onopen = function() {
            console.log('WebSocket connection established');
            updateConnectionStatus('Connected', 'success');
        };
        
        websocket.onmessage = function(event) {
            try {
                const data = JSON.parse(event.data);
                handleWebSocketMessage(data);
            } catch (error) {
                console.error('Failed to parse WebSocket message:', error);
            }
        };
        
        websocket.onclose = function(event) {
            console.log('WebSocket connection closed:', event.code, event.reason);
            updateConnectionStatus('Disconnected', 'danger');
            
            // Don't reconnect immediately if it was a normal close
            if (event.code !== 1000) {
                // Attempt to reconnect after 5 seconds for abnormal closes
                setTimeout(initializeWebSocket, 5000);
            }
        };
        
        websocket.onerror = function(error) {
            console.error('WebSocket error:', error);
            updateConnectionStatus('Connection Error', 'warning');
        };
        
        // Handle connection cleanup on page unload
        window.addEventListener('beforeunload', function() {
            if (websocket && websocket.readyState === WebSocket.OPEN) {
                websocket.close(1000, 'Page unloading');
            }
        });
        
    } catch (error) {
        console.error('WebSocket connection failed:', error);
        updateConnectionStatus('WebSocket Unavailable', 'warning');
    }
}

function handleWebSocketMessage(data) {
    if (data.type === 'state_update') {
        // Update state display if currently viewing state tab
        if (currentTab === 'state-tab') {
            displayState(data.data);
        }
    } else if (data.type === 'processing_started') {
        updateStatus(`Processing: ${data.request}`);
        showLoader();
    } else if (data.type === 'processing_completed') {
        if (data.success) {
            updateStatus('Processing completed successfully');
        } else {
            updateStatus('Processing failed');
        }
        hideLoader();
    } else if (data.type === 'execution_started') {
        addExecutionLog('info', `Execution started for decision: ${data.decisionId}`);
        updateExecutionStatus('Initializing execution...');
    } else if (data.type === 'step_started') {
        addExecutionLog('info', data.message);
        updateStepStatus(data.stepId, 'running');
    } else if (data.type === 'step_progress') {
        addExecutionLog('info', data.message);
    } else if (data.type === 'step_completed') {
        addExecutionLog('success', data.message);
        updateStepStatus(data.stepId, 'completed');
    } else if (data.type === 'step_failed') {
        addExecutionLog('error', data.message);
        updateStepStatus(data.stepId, 'failed');
    } else if (data.type === 'execution_completed') {
        addExecutionLog('success', data.message);
        updateExecutionStatus('Execution completed');
        updateExecutionProgress(100);
        stopExecutionTimer();
    }
}

function updateConnectionStatus(message, type) {
    const statusEl = document.getElementById('connectionStatus');
    const iconClass = type === 'success' ? 'fa-circle text-success' : 
                     type === 'warning' ? 'fa-exclamation-circle text-warning' : 
                     'fa-times-circle text-danger';
    
    statusEl.innerHTML = `<i class="fas ${iconClass}"></i> ${message}`;
}

// Tab Management
function openTab(evt, tabName) {
    // Hide all tab contents
    const tabContents = document.getElementsByClassName('tab-content');
    for (let i = 0; i < tabContents.length; i++) {
        tabContents[i].classList.remove('active');
    }
    
    // Remove active class from all tab buttons
    const tabButtons = document.getElementsByClassName('tab-button');
    for (let i = 0; i < tabButtons.length; i++) {
        tabButtons[i].classList.remove('active');
    }
    
    // Show selected tab and mark button as active
    document.getElementById(tabName).classList.add('active');
    evt.currentTarget.classList.add('active');
    
    currentTab = tabName;
    
    // Load content if needed
    switch (tabName) {
        case 'state-tab':
            loadState();
            break;
        case 'graph-tab':
            // Don't auto-load graph as it might be expensive
            break;
        case 'conflicts-tab':
            // Don't auto-load conflicts
            break;
        case 'plan-tab':
            // Don't auto-load plan
            break;
    }
}

// Utility Functions
function showLoader() {
    document.getElementById('loader').style.display = 'block';
}

function hideLoader() {
    document.getElementById('loader').style.display = 'none';
}

function updateStatus(message) {
    document.getElementById('statusMessage').textContent = message;
}

function showError(message) {
    updateStatus(`Error: ${message}`);
    hideLoader();
}

async function apiCall(endpoint, options = {}) {
    try {
        showLoader();
        updateStatus(`Calling ${endpoint}...`);
        
        const response = await fetch(`/api${endpoint}`, {
            headers: {
                'Content-Type': 'application/json',
                ...options.headers
            },
            ...options
        });
        
        if (!response.ok) {
            throw new Error(`HTTP ${response.status}: ${response.statusText}`);
        }
        
        const responseText = await response.text();
        
        // Check if response is empty
        if (!responseText) {
            throw new Error('Empty response from server');
        }
        
        // Try to parse JSON
        let data;
        try {
            data = JSON.parse(responseText);
        } catch (jsonError) {
            console.error('JSON parse error:', jsonError);
            console.error('Response text:', responseText);
            throw new Error(`Invalid JSON response: ${jsonError.message}`);
        }
        
        updateStatus('Ready');
        return data;
    } catch (error) {
        showError(error.message);
        throw error;
    } finally {
        hideLoader();
    }
}

// AI Agent Functions
async function processAgentRequest() {
    const request = document.getElementById('agentRequest').value.trim();
    if (!request) {
        showError('Please enter a request');
        return;
    }
    
    const dryRun = document.getElementById('dryRun').checked;
    
    try {
        const response = await apiCall('/agent/process', {
            method: 'POST',
            body: JSON.stringify({ request, dry_run: dryRun })
        });
        
        displayAgentResponse(response);
    } catch (error) {
        console.error('Agent request failed:', error);
    }
}

function displayAgentResponse(response) {
    console.log('Displaying agent response:', response); // Debug log
    const responseArea = document.getElementById('agentResponse');
    const resultDiv = document.getElementById('agentResult');
    
    let html = `
        <div class="alert alert-info">
            <h4><i class="fas fa-robot"></i> AI Agent Response</h4>
            <p><strong>Request:</strong> ${response.request}</p>
            <p><strong>Mode:</strong> ${response.mode === 'demo' ? 'Demo Mode' : (response.dry_run ? 'Dry Run' : 'Live Execution')}</p>
            <p><strong>Confidence:</strong> ${(response.confidence * 100).toFixed(1)}%</p>
        </div>
    `;
    
    if (response.mode === 'demo') {
        // Demo mode response
        html += `
            <div class="data-item">
                <h4>Response</h4>
                <div class="content">${response.response}</div>
            </div>
        `;
    } else if (response.mode === 'live') {
        // Live mode response - show decision and execution details
        html += `
            <div class="data-item">
                <h4>Action</h4>
                <div class="content">${response.action}</div>
            </div>
            
            <div class="data-item">
                <h4>Reasoning</h4>
                <div class="content">${response.reasoning}</div>
            </div>
        `;
        
        // Only show execution details if execution has started
        if (response.execution) {
            html += `
                <div class="data-item">
                    <h4>Execution Status</h4>
                    <div class="content">${response.execution.status}</div>
                </div>
            `;
            
            if (response.execution.actions && response.execution.actions.length > 0) {
                html += `
                    <div class="data-item">
                        <h4>Actions Performed</h4>
                        <ul>
                            ${response.execution.actions.map(action => `<li>${action}</li>`).join('')}
                        </ul>
                    </div>
                `;
            }
            
            if (response.execution.errors && response.execution.errors.length > 0) {
                html += `
                    <div class="data-item">
                        <h4>Errors</h4>
                        <ul class="error-list">
                            ${response.execution.errors.map(error => `<li>${error}</li>`).join('')}
                        </ul>
                    </div>
                `;
            }
        } else if (response.requiresConfirmation) {
            html += `
                <div class="data-item">
                    <h4>Status</h4>
                    <div class="content">Plan generated - waiting for confirmation</div>
                </div>
            `;
        }

        // Display execution plan if available
        console.log('Checking execution plan:', response.executionPlan); // Debug log
        if (response.executionPlan && response.executionPlan.length > 0) {
            console.log('Displaying execution plan with', response.executionPlan.length, 'steps'); // Debug log
            // Use setTimeout to ensure DOM is updated before showing plan
            setTimeout(() => {
                displayExecutionPlan(response.executionPlan, response.decision);
            }, 100);
        } else {
            console.log('No execution plan found in response'); // Debug log
        }
    }
    
    console.log('Setting result HTML:', html); // Debug log
    resultDiv.innerHTML = html;
    responseArea.style.display = 'block';
}

// State Management Functions
async function loadState() {
    const contentEl = document.getElementById('stateContent');
    const refreshButton = document.querySelector('button[onclick="loadState()"]');
    
    try {
        // Show loading state
        if (refreshButton) {
            refreshButton.disabled = true;
            refreshButton.innerHTML = '<i class="fas fa-spinner fa-spin"></i> Refreshing...';
        }
        contentEl.innerHTML = '<div class="loading">Refreshing state...</div>';
        
        console.log('Loading state from /api/state...');
        const state = await apiCall('/state');
        console.log('State loaded successfully:', state);
        displayState(state);
        
        // Show success feedback
        if (refreshButton) {
            refreshButton.innerHTML = '<i class="fas fa-check"></i> Refreshed';
            setTimeout(() => {
                refreshButton.innerHTML = '<i class="fas fa-sync-alt"></i> Refresh State';
                refreshButton.disabled = false;
            }, 1000);
        }
        
    } catch (error) {
        console.error('Failed to load state:', error);
        
        // Reset button state
        if (refreshButton) {
            refreshButton.innerHTML = '<i class="fas fa-sync-alt"></i> Refresh State';
            refreshButton.disabled = false;
        }
        
        // Display error in UI
        contentEl.innerHTML = `
            <div class="error">
                <strong>Failed to load state:</strong> ${error.message}
                <br><br>
                This might indicate that the server is not running or there's a configuration issue.
                <br><br>
                <button class="btn btn-secondary" onclick="loadState()">Retry</button>
            </div>
        `;
    }
}

function displayState(state) {
    const contentEl = document.getElementById('stateContent');
    
    // Handle both direct state and managed_state structure
    let actualState = state;
    if (state.managed_state) {
        actualState = state.managed_state;
    }
    
    // Handle string responses (JSON as string)
    if (typeof actualState === 'string') {
        try {
            actualState = JSON.parse(actualState);
        } catch (e) {
            console.error('Failed to parse state JSON:', e);
            contentEl.innerHTML = '<div class="error">Invalid state data received</div>';
            return;
        }
    }
    
    const resourceCount = Object.keys(actualState.resources || {}).length;
    
    let html = `
        <div class="stats-grid">
            <div class="stat-card">
                <span class="number">${resourceCount}</span>
                <span class="label">Managed Resources</span>
            </div>
            <div class="stat-card">
                <span class="number">${actualState.version || '1.0.0'}</span>
                <span class="label">State Version</span>
            </div>
            <div class="stat-card">
                <span class="number">${new Date(actualState.lastUpdated || actualState.last_updated || Date.now()).toLocaleDateString()}</span>
                <span class="label">Last Updated</span>
            </div>
            <div class="stat-card">
                <span class="number">${actualState.region || 'N/A'}</span>
                <span class="label">Region</span>
            </div>
        </div>
    `;
    
    if (actualState.error) {
        html += `<div class="error">Error: ${actualState.error}</div>`;
    }
    
    if (resourceCount > 0) {
        html += '<div class="data-grid">';
        for (const [id, resource] of Object.entries(actualState.resources)) {
            // Extract AWS resource ID from MCP response if available
            let awsResourceId = 'N/A';
            if (resource.properties && resource.properties.mcp_response) {
                const mcpResponse = resource.properties.mcp_response;
                awsResourceId = mcpResponse.groupId || mcpResponse.instanceId || mcpResponse.vpcId || mcpResponse.subnetId || 'N/A';
            }
            
            html += `
                <div class="data-item">
                    <h4>${id}</h4>
                    <div class="meta">
                        <span class="badge type-${resource.type}">${resource.type}</span>
                        <span class="badge status-${resource.status}">${resource.status}</span>
                        ${awsResourceId !== 'N/A' ? `<span class="badge aws-id">AWS: ${awsResourceId}</span>` : ''}
                    </div>
                    <div class="details">
                        <div class="detail-row">
                            <strong>Name:</strong> ${resource.name || 'Unnamed'}
                        </div>
                        <div class="detail-row">
                            <strong>Created:</strong> ${new Date(resource.createdAt || Date.now()).toLocaleString()}
                        </div>
                        ${resource.dependencies && resource.dependencies.length > 0 ? 
                            `<div class="detail-row"><strong>Dependencies:</strong> ${resource.dependencies.join(', ')}</div>` : ''
                        }
                    </div>
                    <details class="properties-details">
                        <summary>Properties</summary>
                        <pre class="properties-content">${JSON.stringify(resource.properties, null, 2)}</pre>
                    </details>
                </div>
            `;
        }
        html += '</div>';
    } else {
        html += '<div class="placeholder">No managed resources found. Create some infrastructure to see it here.</div>';
    }
    
    contentEl.innerHTML = html;
}

async function discoverInfrastructure() {
    try {
        const result = await apiCall('/discover', { method: 'POST' });
        displayDiscoveredResources(result);
    } catch (error) {
        console.error('Discovery failed:', error);
    }
}

function displayDiscoveredResources(result) {
    const contentEl = document.getElementById('stateContent');
    
    let html = `
        <div class="alert alert-success">
            <h4><i class="fas fa-search"></i> Infrastructure Discovery Complete</h4>
            <p>Found ${result.count} resources at ${new Date(result.timestamp).toLocaleString()}</p>
        </div>
    `;
    
    if (result.resources && result.resources.length > 0) {
        html += '<div class="data-grid">';
        result.resources.forEach(resource => {
            html += `
                <div class="data-item">
                    <h4>${resource.id}</h4>
                    <div class="meta">Type: ${resource.type} | Status: ${resource.status}</div>
                    <div class="content">${JSON.stringify(resource.properties, null, 2)}</div>
                </div>
            `;
        });
        html += '</div>';
    } else {
        html += '<div class="placeholder">No resources discovered</div>';
    }
    
    contentEl.innerHTML = html;
}

async function exportState() {
    try {
        const includeDiscovered = confirm('Include discovered (unmanaged) resources in export?');
        const url = `/api/export?include_discovered=${includeDiscovered}`;
        
        const response = await fetch(url);
        if (!response.ok) {
            throw new Error(`Export failed: ${response.statusText}`);
        }
        
        const blob = await response.blob();
        const downloadUrl = window.URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = downloadUrl;
        a.download = `infrastructure-state-${new Date().toISOString().split('T')[0]}.json`;
        document.body.appendChild(a);
        a.click();
        document.body.removeChild(a);
        window.URL.revokeObjectURL(downloadUrl);
        
        updateStatus('State exported successfully');
    } catch (error) {
        showError(`Export failed: ${error.message}`);
    }
}

// Graph Functions
async function loadGraph() {
    try {
        const format = document.getElementById('graphFormat').value;
        const result = await apiCall(`/graph?format=${format}`);
        displayGraph(result);
    } catch (error) {
        console.error('Graph generation failed:', error);
    }
}

function displayGraph(result) {
    const contentEl = document.getElementById('graphContent');
    
    let html = `
        <div class="alert alert-info">
            <h4><i class="fas fa-project-diagram"></i> Dependency Graph</h4>
            <p>Format: ${result.format} | Generated: ${new Date(result.timestamp).toLocaleString()}</p>
        </div>
    `;
    
    if (result.format === 'mermaid') {
        html += `
            <div class="mermaid">
                ${result.visualization}
            </div>
        `;
    } else {
        html += `
            <div class="graph-text">${result.visualization}</div>
        `;
    }
    
    if (result.bottlenecks && result.bottlenecks.length > 0) {
        html += `
            <div class="alert alert-warning">
                <h4><i class="fas fa-exclamation-triangle"></i> Bottlenecks Detected</h4>
                <ul>
                    ${result.bottlenecks.map(b => `<li>${b}</li>`).join('')}
                </ul>
            </div>
        `;
    }
    
    contentEl.innerHTML = html;
    
    // Re-render mermaid diagrams
    if (result.format === 'mermaid') {
        mermaid.init(undefined, contentEl.querySelector('.mermaid'));
    }
}

// Conflicts Functions
async function loadConflicts() {
    try {
        const autoResolve = document.getElementById('autoResolve').checked;
        const result = await apiCall(`/conflicts?auto_resolve=${autoResolve}`);
        displayConflicts(result);
    } catch (error) {
        console.error('Conflict detection failed:', error);
    }
}

function displayConflicts(result) {
    const contentEl = document.getElementById('conflictsContent');
    
    let html = `
        <div class="alert ${result.count > 0 ? 'alert-warning' : 'alert-success'}">
            <h4><i class="fas fa-exclamation-triangle"></i> Conflict Analysis</h4>
            <p>Found ${result.count} conflicts | Analyzed: ${new Date(result.timestamp).toLocaleString()}</p>
        </div>
    `;
    
    if (result.conflicts && result.conflicts.length > 0) {
        html += '<div class="data-grid">';
        result.conflicts.forEach(conflict => {
            html += `
                <div class="data-item conflict-item">
                    <h4>Conflict: ${conflict.resource_id}</h4>
                    <div class="meta">Type: ${conflict.type} | Severity: ${conflict.severity}</div>
                    <div class="content">${conflict.description}</div>
                </div>
            `;
        });
        html += '</div>';
    }
    
    if (result.resolutions && result.resolutions.length > 0) {
        html += '<h3 style="margin: 20px; color: #28a745;">Auto-Resolutions</h3>';
        html += '<div class="data-grid">';
        result.resolutions.forEach(resolution => {
            html += `
                <div class="data-item resolution-item">
                    <h4>Resolution: ${resolution.conflict.resource_id}</h4>
                    <div class="meta">Action: ${resolution.resolution.action}</div>
                    <div class="content">${resolution.resolution.description}</div>
                </div>
            `;
        });
        html += '</div>';
    }
    
    if (result.count === 0) {
        html += '<div class="placeholder">No conflicts detected</div>';
    }
    
    contentEl.innerHTML = html;
}

// Planning Functions
async function generatePlan() {
    try {
        const includeLevels = document.getElementById('includeLevels').checked;
        const result = await apiCall('/plan', {
            method: 'POST',
            body: JSON.stringify({ include_levels: includeLevels })
        });
        displayPlan(result);
    } catch (error) {
        console.error('Plan generation failed:', error);
    }
}

function displayPlan(result) {
    const contentEl = document.getElementById('planContent');
    
    let html = `
        <div class="alert alert-info">
            <h4><i class="fas fa-calendar-alt"></i> Deployment Plan</h4>
            <p>Resources: ${result.resource_count} | Generated: ${new Date(result.timestamp).toLocaleString()}</p>
        </div>
    `;
    
    if (result.deployment_order && result.deployment_order.length > 0) {
        html += `
            <div class="data-item">
                <h4>Deployment Order</h4>
                <div class="content">
                    ${result.deployment_order.map((resource, index) => 
                        `${index + 1}. ${resource}`
                    ).join('\n')}
                </div>
            </div>
        `;
    }
    
    if (result.deployment_levels) {
        html += `
            <div class="data-item">
                <h4>Deployment Levels (Parallel Groups)</h4>
                <div class="content">
                    ${Object.entries(result.deployment_levels).map(([level, resources]) => 
                        `Level ${level}: ${resources.join(', ')}`
                    ).join('\n')}
                </div>
            </div>
        `;
    }
    
    if (!result.deployment_order || result.deployment_order.length === 0) {
        html += '<div class="placeholder">No deployment plan available</div>';
    }
    
    contentEl.innerHTML = html;
}

// Keyboard shortcuts
document.addEventListener('keydown', function(e) {
    // Ctrl+Enter to process agent request
    if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') {
        if (currentTab === 'agent-tab') {
            processAgentRequest();
        }
    }
    
    // F5 to refresh current tab content
    if (e.key === 'F5') {
        e.preventDefault();
        switch (currentTab) {
            case 'state-tab':
                loadState();
                break;
            case 'graph-tab':
                loadGraph();
                break;
            case 'conflicts-tab':
                loadConflicts();
                break;
            case 'plan-tab':
                generatePlan();
                break;
        }
    }
});

// Execution Plan Functions
let currentPlan = null;
let currentDecision = null;
let executionStartTime = null;
let executionTimer = null;

function displayExecutionPlan(executionPlan, decision) {
    console.log('displayExecutionPlan called with:', { executionPlan, decision }); // Debug log
    currentPlan = executionPlan;
    currentDecision = decision;
    
    const planDiv = document.getElementById('executionPlan');
    const stepsDiv = document.getElementById('planSteps');
    const confirmBtn = document.getElementById('confirmPlanBtn');
    
    console.log('Plan display elements:', { planDiv, stepsDiv, confirmBtn }); // Debug log
    
    let html = '';
    executionPlan.forEach((step, index) => {
        html += `
            <div class="plan-step" data-step-id="${step.id}">
                <div class="step-number">${index + 1}</div>
                <div class="step-content">
                    <div class="step-name">${step.name}</div>
                    <div class="step-description">${step.description}</div>
                    <div class="step-meta">
                        <span class="step-duration">⏱️ ${step.estimatedDuration}</span>
                        <span class="step-action">🔧 ${step.action}</span>
                        <span class="step-resource">📦 ${step.resourceId}</span>
                        ${step.dependsOn && step.dependsOn.length > 0 ? 
                            `<span class="step-dependencies">🔗 Depends on: ${step.dependsOn.join(', ')}</span>` : ''}
                    </div>
                </div>
                <div class="step-status pending">Pending</div>
            </div>
        `;
    });
    
    stepsDiv.innerHTML = html;
    confirmBtn.style.display = 'inline-block';
    planDiv.style.display = 'block';
    
    console.log('Execution plan displayed successfully'); // Debug log
}

async function confirmAndExecutePlan() {
    if (!currentDecision) {
        showError('No decision available for execution');
        return;
    }
    
    try {
        // Hide plan, show progress
        document.getElementById('executionPlan').style.display = 'none';
        const progressDiv = document.getElementById('executionProgress');
        progressDiv.style.display = 'block';
        
        // Initialize execution tracking
        executionStartTime = Date.now();
        startExecutionTimer();
        
        // Call execute API
        const response = await apiCall('/agent/execute', {
            method: 'POST',
            body: JSON.stringify({ decisionId: currentDecision.id })
        });
        
        if (response.success) {
            addExecutionLog('info', `Plan execution started: ${response.executionId}`);
            updateExecutionStatus('Execution started...');
        } else {
            addExecutionLog('error', 'Failed to start execution');
            updateExecutionStatus('Execution failed to start');
        }
        
    } catch (error) {
        console.error('Failed to execute plan:', error);
        addExecutionLog('error', `Execution failed: ${error.message}`);
        updateExecutionStatus('Execution error');
    }
}

function cancelPlan() {
    document.getElementById('executionPlan').style.display = 'none';
    currentPlan = null;
    currentDecision = null;
}

function updateExecutionStatus(status) {
    document.getElementById('executionStatus').textContent = status;
}

function addExecutionLog(level, message) {
    const logsDiv = document.getElementById('executionLogs');
    const timestamp = new Date().toLocaleTimeString();
    
    const logEntry = document.createElement('div');
    logEntry.className = 'log-entry';
    logEntry.innerHTML = `
        <span class="timestamp">${timestamp}</span>
        <span class="level-${level}">[${level.toUpperCase()}]</span>
        <span class="message">${message}</span>
    `;
    
    logsDiv.appendChild(logEntry);
    logsDiv.scrollTop = logsDiv.scrollHeight;
}

function updateExecutionProgress(percentage) {
    const progressFill = document.getElementById('progressFill');
    progressFill.style.width = `${percentage}%`;
}

function startExecutionTimer() {
    const timerElement = document.getElementById('executionTimer');
    
    executionTimer = setInterval(() => {
        if (executionStartTime) {
            const elapsed = Date.now() - executionStartTime;
            const minutes = Math.floor(elapsed / 60000);
            const seconds = Math.floor((elapsed % 60000) / 1000);
            timerElement.textContent = `${minutes.toString().padStart(2, '0')}:${seconds.toString().padStart(2, '0')}`;
        }
    }, 1000);
}

function stopExecutionTimer() {
    if (executionTimer) {
        clearInterval(executionTimer);
        executionTimer = null;
    }
}

function updateStepStatus(stepId, status) {
    const stepElement = document.querySelector(`[data-step-id="${stepId}"]`);
    if (stepElement) {
        const statusElement = stepElement.querySelector('.step-status');
        if (statusElement) {
            statusElement.className = `step-status ${status}`;
            statusElement.textContent = status.charAt(0).toUpperCase() + status.slice(1);
        }
    }
}
