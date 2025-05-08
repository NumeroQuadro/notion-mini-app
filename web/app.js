// Initialize Telegram WebApp
const tg = window.Telegram.WebApp;
tg.expand();

// For logging - add this
function logAction(action, data) {
    console.log(`[${new Date().toISOString()}] ${action}:`, data);
    
    // Optional: Send logs to server
    fetch('/notion/mini-app/api/log', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ action, data, timestamp: new Date().toISOString() })
    }).catch(err => console.error('Error logging:', err));
}

// Known checkbox property names - centralized list
const CHECKBOX_PROPERTIES = ['complete', 'status', 'done', 'complete'];

// Initialize Notion Client (initialized on demand)
let notionClient = null;
let appConfig = null;

// Fetch application configuration from server
async function getAppConfig() {
    if (appConfig) {
        return appConfig;
    }
    
    try {
        logAction('Fetching app config', {});
        const response = await fetch('/notion/mini-app/api/config');
        
        if (!response.ok) {
            throw new Error(`Failed to fetch config: ${response.status}`);
        }
        
        appConfig = await response.json();
        logAction('App config loaded', { keys: Object.keys(appConfig) });
        return appConfig;
    } catch (error) {
        logAction('Error loading app config', { error: error.message });
        
        // Return empty config as fallback
        appConfig = { _source: 'fallback' };
        return appConfig;
    }
}

// Initialize Notion client with authentication
async function getNotionClient() {
    // If we already have a client, return it
    if (notionClient) {
        return notionClient;
    }
    
    try {
        // Get configuration from server
        const config = await getAppConfig();
        const apiToken = config.NOTION_API_KEY;
        
        if (!apiToken) {
            logAction('No API token available', { configSource: config._source });
            throw new Error('Notion API key not found in configuration');
        }
        
        // Create and return the Notion client
        if (window.NotionHQClient) {
            // The UMD build exposes the client constructor as NotionHQClient
            notionClient = new window.NotionHQClient({ 
                auth: apiToken
            });
            logAction('Notion client initialized', { success: true });
            return notionClient;
        } else {
            throw new Error('Notion API not available in this browser');
        }
    } catch (error) {
        logAction('Error initializing Notion client', { error: error.message });
        
        // Always return null to indicate failure
        return null;
    }
}

// Get database properties from the backend
async function getDatabaseProperties() {
    try {
        logAction('Fetching properties', {});
        const response = await fetch('/notion/mini-app/api/properties');
        
        if (!response.ok) {
            const errorText = await response.text();
            throw new Error(`API returned ${response.status}: ${errorText}`);
        }
        
        const properties = await response.json();
        logAction('Properties fetched', properties);
        
        // Handle checkbox properties specially
        for (const [key, config] of Object.entries(properties)) {
            // Convert checkbox properties to proper checkbox type
            if (CHECKBOX_PROPERTIES.includes(key.toLowerCase())) {
                logAction('Marking checkbox property', { key });
                properties[key].type = 'checkbox';
            }
        }
        
        return properties;
    } catch (error) {
        console.error('Error fetching properties:', error);
        logAction('Error fetching properties', { error: error.message });
        
        // Return default properties if API fails
        return {
            "Name": { type: "title", required: true },
            "Tags": { type: "multi_select", options: ["sometimes-later"] },
            "project": { type: "select", options: ["household-tasks", "the-wellness-hub"] },
            "Date": { type: "date" }
        };
    }
}

// Create form fields based on Notion properties
async function createPropertyFields() {
    const properties = await getDatabaseProperties();
    const container = document.getElementById('propertiesContainer');
    
    logAction('Creating form fields', { properties });

    // Check if we're on a mobile device
    const isMobile = /iPhone|iPad|iPod|Android/i.test(navigator.userAgent) || 
                    (window.Telegram && window.Telegram.WebApp);
    
    // Clear container first
    container.innerHTML = '';

    // Create fields for each property
    for (const [key, config] of Object.entries(properties)) {
        // Skip the 'Name' property since we already have a title field
        if (key === 'Name' || key === 'title' || config.type === 'title') {
            continue;
        }
        
        // Skip button-like properties
        const buttonKeywords = ['button', 'submit', 'action'];
        if (buttonKeywords.some(keyword => key.toLowerCase().includes(keyword))) {
            logAction('Skipping button-like property', { key });
            continue;
        }

        const formGroup = document.createElement('div');
        formGroup.className = 'form-group';

        const label = document.createElement('label');
        label.htmlFor = key;
        label.textContent = key;
        if (config.required) {
            const required = document.createElement('span');
            required.textContent = ' *';
            required.style.color = 'red';
            label.appendChild(required);
        }

        // Skip creating input fields for button properties only
        if (config.type === 'button') {
            logAction('Skipping button property in form', { key });
            continue;
        }

        let input;
        switch (config.type) {
            case 'multi_select':
                // Create checkboxes for multi-select
                const checkboxContainer = document.createElement('div');
                checkboxContainer.className = 'checkbox-group';
                
                if (config.options && config.options.length > 0) {
                    // On mobile, if there are many options, use a select with multiple instead
                    if (isMobile && config.options.length > 5) {
                        const select = document.createElement('select');
                        select.multiple = true;
                        select.id = key;
                        select.name = key;
                        select.dataset.type = 'multi_select';
                        select.dataset.propName = key;
                        
                        // Add a helper text
                        const helpText = document.createElement('div');
                        helpText.className = 'help-text';
                        helpText.textContent = 'Tap multiple items to select them';
                        helpText.style.fontSize = '12px';
                        helpText.style.color = '#666';
                        helpText.style.marginBottom = '5px';
                        
                        checkboxContainer.appendChild(helpText);
                        
                        config.options.forEach(option => {
                            const optionElement = document.createElement('option');
                            optionElement.value = option;
                            optionElement.textContent = option;
                            select.appendChild(optionElement);
                        });
                        
                        checkboxContainer.appendChild(select);
                    } else {
                        // Use regular checkboxes for fewer options or desktop
                        config.options.forEach(option => {
                            const checkboxDiv = document.createElement('div');
                            
                            const checkbox = document.createElement('input');
                            checkbox.type = 'checkbox';
                            checkbox.id = `${key}-${option}`;
                            checkbox.name = key;
                            checkbox.value = option;
                            checkbox.dataset.type = 'multi_select';
                            
                            const checkboxLabel = document.createElement('label');
                            checkboxLabel.htmlFor = `${key}-${option}`;
                            checkboxLabel.textContent = option;
                            
                            // Make touch targets larger on mobile
                            if (isMobile) {
                                checkboxDiv.style.padding = '8px 0';
                                checkbox.style.width = '24px';
                                checkbox.style.height = '24px';
                            }
                            
                            checkboxDiv.appendChild(checkbox);
                            checkboxDiv.appendChild(checkboxLabel);
                            checkboxContainer.appendChild(checkboxDiv);
                        });
                    }
                } else {
                    // If no options provided, provide a text input with comma separation
                    const textInput = document.createElement('input');
                    textInput.type = 'text';
                    textInput.id = key;
                    textInput.placeholder = 'Enter comma-separated values';
                    textInput.dataset.type = 'multi_select_text';
                    textInput.dataset.propName = key;
                    checkboxContainer.appendChild(textInput);
                }
                
                input = checkboxContainer;
                break;
                
            case 'select':
                if (config.options && config.options.length > 0) {
                    // For mobile or many options, use a native select element
                    if (isMobile || config.options.length > 5) {
                        input = document.createElement('select');
                        input.id = key;
                        input.name = key;
                        input.dataset.type = 'select';
                        input.dataset.propName = key;
                        
                        // Add an empty option
                        const emptyOption = document.createElement('option');
                        emptyOption.value = '';
                        emptyOption.textContent = '-- Select --';
                        input.appendChild(emptyOption);
                        
                        config.options.forEach(option => {
                            const optionElement = document.createElement('option');
                            optionElement.value = option;
                            optionElement.textContent = option;
                            input.appendChild(optionElement);
                        });
                    } else {
                        // Use radio buttons for desktop with fewer options
                        input = document.createElement('div');
                        input.className = 'radio-group';
                        
                        config.options.forEach(option => {
                            const radioDiv = document.createElement('div');
                            
                            const radio = document.createElement('input');
                            radio.type = 'radio';
                            radio.id = `${key}-${option}`;
                            radio.name = key;
                            radio.value = option;
                            radio.dataset.type = 'select';
                            
                            const radioLabel = document.createElement('label');
                            radioLabel.htmlFor = `${key}-${option}`;
                            radioLabel.textContent = option;
                            
                            // Make touch targets larger on mobile
                            if (isMobile) {
                                radioDiv.style.padding = '8px 0';
                                radio.style.width = '24px';
                                radio.style.height = '24px';
                            }
                            
                            radioDiv.appendChild(radio);
                            radioDiv.appendChild(radioLabel);
                            input.appendChild(radioDiv);
                        });
                    }
                } else {
                    // Fallback to a select dropdown
                    input = document.createElement('select');
                    input.id = key;
                    input.name = key;
                    input.dataset.type = 'select';
                    input.dataset.propName = key;
                    
                    // Add an empty option
                    const emptyOption = document.createElement('option');
                    emptyOption.value = '';
                    emptyOption.textContent = '-- Select --';
                    input.appendChild(emptyOption);
                }
                break;

            case 'checkbox':
                const checkboxWrapper = document.createElement('div');
                checkboxWrapper.style.display = 'flex';
                checkboxWrapper.style.alignItems = 'center';
                
                input = document.createElement('input');
                input.type = 'checkbox';
                input.id = key;
                input.name = key;
                input.value = 'true';
                input.dataset.type = 'checkbox';
                input.dataset.propName = key;
                
                // Make touch target larger on mobile
                if (isMobile) {
                    input.style.width = '24px';
                    input.style.height = '24px';
                    input.style.marginRight = '10px';
                }
                
                const inlineLabel = document.createElement('label');
                inlineLabel.htmlFor = key;
                inlineLabel.textContent = 'Yes';
                inlineLabel.style.marginLeft = '8px';
                inlineLabel.style.display = 'inline-block';
                
                checkboxWrapper.appendChild(input);
                checkboxWrapper.appendChild(inlineLabel);
                
                input = checkboxWrapper;
                break;
                
            case 'date':
                input = document.createElement('input');
                input.type = 'date';
                input.id = key;
                input.dataset.type = 'date';
                input.dataset.propName = key;
                
                // Add better date picker for mobile
                if (isMobile) {
                    input.onfocus = function() {
                        this.showPicker();
                    };
                }
                break;

            case 'url':
                input = document.createElement('input');
                input.type = 'url'; 
                input.id = key;
                input.placeholder = 'https://';
                input.dataset.type = 'url';
                input.dataset.propName = key;
                
                // Add keyboard type for mobile
                if (isMobile) {
                    input.setAttribute('inputmode', 'url');
                }
                break;

            case 'email':
                input = document.createElement('input');
                input.type = 'email';
                input.id = key;
                input.dataset.type = 'email';
                input.dataset.propName = key;
                
                // Add keyboard type for mobile
                if (isMobile) {
                    input.setAttribute('inputmode', 'email');
                }
                break;

            case 'phone_number':
                input = document.createElement('input');
                input.type = 'tel';
                input.id = key;
                input.dataset.type = 'phone_number';
                input.dataset.propName = key;
                
                // Add keyboard type for mobile
                if (isMobile) {
                    input.setAttribute('inputmode', 'tel');
                }
                break;

            case 'number':
                input = document.createElement('input');
                input.type = 'number';
                input.id = key;
                input.dataset.type = 'number';
                input.dataset.propName = key;
                
                // Add keyboard type for mobile
                if (isMobile) {
                    input.setAttribute('inputmode', 'numeric');
                    input.step = 'any'; // Allow decimals
                }
                break;
                
            default:
                input = document.createElement('input');
                input.type = 'text';
                input.id = key;
                input.dataset.type = 'text';
                input.dataset.propName = key;
        }

        input.required = config.required || false;

        formGroup.appendChild(label);
        formGroup.appendChild(input);
        container.appendChild(formGroup);
    }
    
    logAction('Form fields created', {});
}

// Helper function for showing popups with fallback
function showPopup(title, message) {
    // Don't try to use Telegram's native popup as it's not supported
    // Just use alert directly
    console.log(`${title}: ${message}`);
    
    // Show the error in the UI
    const errorContainer = document.getElementById('error-container');
    if (errorContainer) {
        errorContainer.textContent = `${title}: ${message}`;
        errorContainer.style.display = 'block';
        
        // Hide after 5 seconds
        setTimeout(() => {
            errorContainer.style.display = 'none';
        }, 5000);
    } else {
        // Fallback to alert if error container not found
        alert(title + ": " + message);
    }
}

// Format date in YYYY-MM-DD format for Notion
function formatDateForNotion(dateStr) {
    try {
        const date = new Date(dateStr);
        return date.toISOString().split('T')[0]; // Returns YYYY-MM-DD
    } catch (e) {
        console.error("Error formatting date:", e);
        return dateStr; // Return original if parsing fails
    }
}

// Convert form data to Notion properties structure
function convertToNotionProperties(formData) {
    const properties = {
        "Name": {
            title: [
                {
                    text: {
                        content: formData.title
                    }
                }
            ]
        }
    };
    
    // Process each property based on its type
    Object.entries(formData.properties).forEach(([key, value]) => {
        // Skip empty values
        if (value === undefined || value === null || value === '') {
            return;
        }
        
        // Get property type from the form element
        const formElement = document.querySelector(`[data-prop-name="${key}"]`);
        if (!formElement) return;
        
        const propType = formElement.dataset.type;
        
        switch (propType) {
            case 'multi_select':
            case 'multi_select_text':
                // Handle array of values for multi-select
                const options = Array.isArray(value) ? value : [value];
                properties[key] = {
                    multi_select: options.map(opt => ({ name: opt }))
                };
                break;
                
            case 'select':
                properties[key] = {
                    select: { name: value }
                };
                break;
                
            case 'checkbox':
                properties[key] = {
                    checkbox: value === true || value === 'true' || value === 'yes' || value === '1'
                };
                break;
                
            case 'date':
                properties[key] = {
                    date: {
                        start: formatDateForNotion(value)
                    }
                };
                break;
                
            case 'number':
                properties[key] = {
                    number: parseFloat(value)
                };
                break;
                
            case 'url':
                properties[key] = {
                    url: value
                };
                break;
                
            case 'email':
                properties[key] = {
                    email: value
                };
                break;
                
            case 'phone_number':
                properties[key] = {
                    phone_number: value
                };
                break;
                
            default:
                // Default to rich text
                properties[key] = {
                    rich_text: [
                        {
                            text: {
                                content: value
                            }
                        }
                    ]
                };
        }
    });
    
    return properties;
}

// Before sending taskData to the server, filter out button properties
function filterButtonProperties(taskData) {
    const filtered = { ...taskData };
    const buttonKeywords = ['button', 'submit', 'action'];
    
    // Filter out properties with button-like names
    Object.keys(filtered.properties).forEach(key => {
        const lowercaseKey = key.toLowerCase();
        if (buttonKeywords.some(keyword => lowercaseKey.includes(keyword))) {
            console.log(`Filtering out button-like property: ${key}`);
            delete filtered.properties[key];
        }
    });
    
    return filtered;
}

// Handle form submission
async function handleSubmit(event) {
    event.preventDefault();
    logAction('Form submitted', { formId: event.target.id });

    // Get submit button and disable it
    const submitBtn = document.getElementById('submitBtn');
    submitBtn.disabled = true;
    submitBtn.textContent = 'Creating Task...';
    
    // Hide any previous errors
    const errorContainer = document.getElementById('error-container');
    if (errorContainer) {
        errorContainer.style.display = 'none';
    }

    // Get database properties to identify button types
    let dbProperties;
    try {
        dbProperties = await getDatabaseProperties();
        logAction('Retrieved database properties for task creation', { propertyCount: Object.keys(dbProperties).length });
    } catch (error) {
        console.error('Error fetching database properties:', error);
        // Continue anyway, we'll filter out known button properties
    }

    // Create task data object
    const taskData = {
        title: document.getElementById('taskTitle').value.trim(),
        properties: {}
    };
    
    // Validate title
    if (!taskData.title) {
        showPopup('Error', 'Task title is required');
        
        // Reset button
        submitBtn.disabled = false;
        submitBtn.textContent = 'Create Task';
        return;
    }

    // Get form elements
    const form = event.target;
    const formElements = form.elements;
    
    // Process each form element
    for (let i = 0; i < formElements.length; i++) {
        const element = formElements[i];
        
        // Skip the task title and submit button
        if (element.id === 'taskTitle' || element.type === 'submit') {
            continue;
        }
        
        // Handle different input types
        if (element.type === 'checkbox') {
            if (element.checked) {
                // For multi-select, add to array if not exists
                const name = element.name;
                if (name !== element.id) { // multi-select checkbox
                    if (!taskData.properties[name]) {
                        taskData.properties[name] = [];
                    }
                    taskData.properties[name].push(element.value);
                } else { // regular checkbox
                    taskData.properties[element.id] = true;
                }
            }
        } else if (element.type === 'radio') {
            if (element.checked) {
                taskData.properties[element.name] = element.value;
            }
        } else if (element.type === 'date') {
            // Format date fields properly
            if (element.value) {
                taskData.properties[element.id] = formatDateForNotion(element.value);
            }
        } else if (element.tagName === 'DIV') {
            // Skip container divs
            continue;
        } else if (element.type === 'button') {
            // Skip actual button elements
            continue;
        } else if (element.tagName === 'SELECT') {
            // For dropdowns
            if (element.value) {
                taskData.properties[element.id] = element.value;
            }
        } else if (element.dataset.type === 'multi_select_text') {
            // Handle the text input for multi-select
            if (element.value) {
                const values = element.value.split(',').map(v => v.trim()).filter(v => v);
                if (values.length > 0) {
                    taskData.properties[element.id] = values;
                }
            }
        } else {
            // For other inputs
            if (element.value) {
                taskData.properties[element.id] = element.value;
            }
        }
    }

    logAction('Task data collected', taskData);

    // Filter out button properties before sending to server
    const filteredTaskData = filterButtonProperties(taskData);
    logAction('Filtered task data', { 
        original: Object.keys(taskData.properties).length, 
        filtered: Object.keys(filteredTaskData.properties).length 
    });

    // Try to use direct Notion API if available
    let useDirectAPI = false;
    
    try {
        const notionClient = await getNotionClient();
        const config = await getAppConfig();
        const databaseId = config.NOTION_DATABASE_ID;
        
        if (notionClient && databaseId) {
            useDirectAPI = true;
            logAction('Using direct Notion API', { databaseId });
            
            // Convert to Notion properties format
            const notionProperties = convertToNotionProperties(filteredTaskData);
            
            // Create page in Notion database
            const response = await notionClient.pages.create({
                parent: { database_id: databaseId },
                properties: notionProperties
            });
            
            logAction('Task created via direct API', { pageId: response.id });
            showPopup('Success', 'Task created successfully!');
            
            // Reset form
            form.reset();
            
            // Re-enable the button
            submitBtn.disabled = false;
            submitBtn.textContent = 'Create Task';
            return;
        }
    } catch (error) {
        logAction('Error using direct Notion API', { error: error.message });
        console.error('Error using direct Notion API:', error);
        // Fall back to server API
        useDirectAPI = false;
    }

    try {
        // Use server API as fallback
        // Use a timeout to handle cases where the server doesn't respond
        const timeoutPromise = new Promise((_, reject) => {
            setTimeout(() => reject(new Error('Request timed out')), 10000);
        });
        
        // Actual fetch request
        const fetchPromise = fetch('/notion/mini-app/api/tasks', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify(filteredTaskData),
        });
        
        // Race the fetch against timeout
        const response = await Promise.race([fetchPromise, timeoutPromise]);

        // Handle the response
        const responseData = await (async () => {
            try {
                const contentType = response.headers.get('content-type');
                if (contentType && contentType.includes('application/json')) {
                    return { type: 'json', data: await response.json() };
                } else {
                    return { type: 'text', data: await response.text() };
                }
            } catch (err) {
                console.error('Error parsing response:', err);
                return { type: 'error', data: 'Could not parse server response' };
            }
        })();

        if (response.ok) {
            logAction('Task created via server API', { 
                status: response.status, 
                data: responseData.data 
            });
            
            showPopup('Success', 'Task created successfully!');
            
            // Reset form
            form.reset();
        } else {
            const errorMsg = responseData.type === 'json' 
                ? (responseData.data.message || JSON.stringify(responseData.data))
                : responseData.data;
            throw new Error(`Server responded with ${response.status}: ${errorMsg}`);
        }
    } catch (error) {
        console.error('Error creating task:', error);
        logAction('Error creating task', { error: error.message });
        
        showPopup('Error', `Failed to create task: ${error.message}`);
    } finally {
        // Re-enable the button in all cases
        submitBtn.disabled = false;
        submitBtn.textContent = 'Create Task';
    }
}

// Initialize the app
document.addEventListener('DOMContentLoaded', () => {
    logAction('App initialized', {});
    createPropertyFields();
    document.getElementById('taskForm').addEventListener('submit', handleSubmit);
}); 