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

// Get database properties from the backend
async function getDatabaseProperties() {
    try {
        logAction('Fetching properties', {});
        const response = await fetch('/notion/mini-app/api/properties');
        const properties = await response.json();
        logAction('Properties fetched', properties);
        return properties;
    } catch (error) {
        console.error('Error fetching properties:', error);
        logAction('Error fetching properties', { error: error.message });
        return {
            // Provide default properties based on your Notion database
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
    try {
        // Try to use Telegram's native popup
        tg.showPopup({
            title: title,
            message: message,
            buttons: [{ type: 'ok' }]
        });
    } catch (e) {
        // Fallback to alert if showPopup is not supported
        console.log("showPopup not supported, using alert instead:", e);
        alert(title + ": " + message);
    }
}

// Handle form submission
async function handleSubmit(event) {
    event.preventDefault();
    logAction('Form submitted', { formId: event.target.id });

    // Create task data object
    const taskData = {
        title: document.getElementById('taskTitle').value,
        properties: {}
    };

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
                if (!taskData.properties[name]) {
                    taskData.properties[name] = [];
                }
                taskData.properties[name].push(element.value);
            }
        } else if (element.type === 'radio') {
            if (element.checked) {
                taskData.properties[element.name] = element.value;
            }
        } else if (element.tagName === 'DIV') {
            // Skip container divs
            continue;
        } else {
            // For text, date, etc.
            taskData.properties[element.id] = element.value;
        }
    }

    logAction('Sending task data', taskData);

    try {
        const response = await fetch('/notion/mini-app/api/tasks', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify(taskData),
        });

        if (response.ok) {
            const result = await response.json();
            logAction('Task created', result);
            
            showPopup('Success', 'Task created successfully!');
            
            // Optional: close the mini app after successful submission
            setTimeout(() => {
                try {
                    tg.close();
                } catch (e) {
                    console.log("tg.close() not supported:", e);
                    // Just continue if close is not supported
                }
            }, 1500);
        } else {
            throw new Error('Failed to create task');
        }
    } catch (error) {
        console.error('Error creating task:', error);
        logAction('Error creating task', { error: error.message });
        
        showPopup('Error', 'Failed to create task. Please try again.');
    }
}

// Initialize the app
document.addEventListener('DOMContentLoaded', () => {
    logAction('App initialized', {});
    createPropertyFields();
    document.getElementById('taskForm').addEventListener('submit', handleSubmit);
}); 