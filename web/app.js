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
const CHECKBOX_PROPERTIES = ['complete', 'status', 'done'];

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
            if (CHECKBOX_PROPERTIES.includes(key.toLowerCase()) && config.type === 'checkbox') {
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

    // Clear container first
    container.innerHTML = '';

    // Create fields for each property
    for (const [key, config] of Object.entries(properties)) {
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

        // Skip creating input fields for unsupported properties
        if (config.type === 'button' || !config.type) {
            logAction('Skipping unsupported property in form', { key });
            continue;
        }

        let input;
        switch (config.type) {
            case 'multi_select':
                // Create checkboxes for multi-select
                const checkboxContainer = document.createElement('div');
                checkboxContainer.className = 'checkbox-group';
                
                if (config.options) {
                    config.options.forEach(option => {
                        const checkboxDiv = document.createElement('div');
                        
                        const checkbox = document.createElement('input');
                        checkbox.type = 'checkbox';
                        checkbox.id = `${key}-${option}`;
                        checkbox.name = key;
                        checkbox.value = option;
                        
                        const checkboxLabel = document.createElement('label');
                        checkboxLabel.htmlFor = `${key}-${option}`;
                        checkboxLabel.textContent = option;
                        
                        checkboxDiv.appendChild(checkbox);
                        checkboxDiv.appendChild(checkboxLabel);
                        checkboxContainer.appendChild(checkboxDiv);
                    });
                }
                
                input = checkboxContainer;
                break;
                
            case 'select':
                input = document.createElement('div');
                input.className = 'radio-group';
                
                if (config.options) {
                    config.options.forEach(option => {
                        const radioDiv = document.createElement('div');
                        
                        const radio = document.createElement('input');
                        radio.type = 'radio';
                        radio.id = `${key}-${option}`;
                        radio.name = key;
                        radio.value = option;
                        
                        const radioLabel = document.createElement('label');
                        radioLabel.htmlFor = `${key}-${option}`;
                        radioLabel.textContent = option;
                        
                        radioDiv.appendChild(radio);
                        radioDiv.appendChild(radioLabel);
                        input.appendChild(radioDiv);
                    });
                }
                break;
                
            case 'date':
                input = document.createElement('input');
                input.type = 'date';
                break;
                
            case 'title':
                // Title is handled separately as taskTitle
                continue;
                
            default:
                input = document.createElement('input');
                input.type = 'text';
        }

        input.id = key;
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
        
        // Skip checkbox properties by name
        if (CHECKBOX_PROPERTIES.includes(element.id.toLowerCase())) {
            logAction('Skipping known checkbox property', { id: element.id });
            continue;
        }
        
        // Skip properties with no type or unsupported type
        if (dbProperties && 
            (!dbProperties[element.id]?.type || 
             !dbProperties[element.name]?.type)) {
            logAction('Skipping property with no type', { id: element.id || element.name });
            continue;
        }
        
        // Handle different input types
        if (element.type === 'checkbox') {
            if (element.checked) {
                // For multi-select, add to array if not exists
                const name = element.name;
                if (!taskData.properties[name]) {
                    taskData.properties[name] = [];
                }
                taskData.properties[name].push(element.value);
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
            logAction('Skipping button element', { id: element.id });
            continue;
        } else {
            // For other inputs
            if (element.value) {
                taskData.properties[element.id] = element.value;
            }
        }
    }

    logAction('Sending task data', taskData);

    try {
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
            body: JSON.stringify(taskData),
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
            logAction('Task created', { 
                status: response.status, 
                data: responseData.data 
            });
            
            showPopup('Success', 'Task created successfully!');
            
            // Reset form
            document.getElementById('taskForm').reset();
            
            // Don't try to close the mini app as it's not reliable
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