// Initialize Telegram WebApp
const tg = window.Telegram.WebApp;
tg.expand();

// Current database type
let currentDbType = "tasks";
let propertiesCache = {};
let isSubmitting = false;

// Known checkbox property names - centralized list
const CHECKBOX_PROPERTIES = ['complete', 'status', 'done', 'complete'];

// Initialize Notion Client (initialized on demand)
let notionClient = null;
let appConfig = null;

// Initialize the app
document.addEventListener('DOMContentLoaded', async function() {
    // Set up tab click handlers
    document.querySelectorAll('.tab').forEach(tab => {
        tab.addEventListener('click', async function() {
            if (isSubmitting) return; // Don't switch tabs during submission
            
            // Update active tab
            document.querySelectorAll('.tab').forEach(t => t.classList.remove('active'));
            this.classList.add('active');
            
            // Get and store database type
            currentDbType = this.getAttribute('data-db-type');
            console.log(`Switched to ${currentDbType} database`);
            
            // Load appropriate properties
            await loadDatabaseProperties();
        });
    });
    
    // Set up form submission
    const form = document.getElementById('taskForm');
    form.addEventListener('submit', handleSubmit);
    
    // Initial properties load
    try {
        const config = await getAppConfig();
        
        // Check if both databases are available
        if (config.HAS_TASKS_DB !== "true" && config.HAS_NOTES_DB !== "true") {
            showError("No Notion databases configured. Please check your settings.");
            return;
        }
        
        // Hide notes tab if not available
        if (config.HAS_NOTES_DB !== "true") {
            document.getElementById('notesTab').style.display = 'none';
        }
        
        // Hide tasks tab if not available
        if (config.HAS_TASKS_DB !== "true") {
            document.getElementById('tasksTab').style.display = 'none';
            // If tasks not available but notes is, switch to notes
            if (config.HAS_NOTES_DB === "true") {
                document.getElementById('notesTab').click();
            }
        }
        
        // Load initial properties
        await loadDatabaseProperties();
    } catch (error) {
        showError(`Failed to initialize: ${error.message}`);
    }
});

// Fetch application configuration from server
async function getAppConfig() {
    if (appConfig) {
        return appConfig;
    }
    
    try {
        const response = await fetch('/notion/mini-app/api/config');
        
        if (!response.ok) {
            throw new Error(`Failed to fetch config: ${response.status}`);
        }
        
        appConfig = await response.json();
        console.log('App config loaded:', appConfig);
        return appConfig;
    } catch (error) {
        console.error('Error loading app config:', error);
        
        // Return empty config as fallback
        appConfig = { _source: 'fallback' };
        return appConfig;
    }
}

// Load database properties from the server with caching
async function loadDatabaseProperties() {
    try {
        // Show loading state
        document.getElementById('propertiesContainer').innerHTML = '<div class="loading-message">Loading properties...</div>';
        
        // Check cache first
        if (propertiesCache[currentDbType]) {
            createPropertyFields(propertiesCache[currentDbType]);
            return;
        }
        
        // Fetch properties for the current database type
        const response = await fetch(`/notion/mini-app/api/properties?db_type=${currentDbType}`);
        
        if (!response.ok) {
            const errorText = await response.text();
            throw new Error(`API returned ${response.status}: ${errorText}`);
        }
        
        const properties = await response.json();
        console.log(`Properties fetched for ${currentDbType}:`, properties);
        
        // Cache the properties
        propertiesCache[currentDbType] = properties;
        
        // Create form fields for the properties
        createPropertyFields(properties);
    } catch (error) {
        console.error('Error fetching properties:', error);
        document.getElementById('propertiesContainer').innerHTML = 
            `<div class="error-message">Failed to load database properties: ${error.message}</div>`;
    }
}

// Create form fields based on Notion properties
function createPropertyFields(properties) {
    const container = document.getElementById('propertiesContainer');
    
    // Clear container first
    container.innerHTML = '';

    // Check if we're on a mobile device
    const isMobile = /iPhone|iPad|iPod|Android/i.test(navigator.userAgent) || 
                    (window.Telegram && window.Telegram.WebApp);
    
    // Create fields for each property
    for (const [key, config] of Object.entries(properties)) {
        // Skip the 'Name' property since we already have a title field
        if (key === 'Name' || key === 'title' || config.type === 'title') {
            continue;
        }
        
        // Skip button-like properties
        const buttonKeywords = ['button', 'submit', 'action'];
        if (buttonKeywords.some(keyword => key.toLowerCase().includes(keyword))) {
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

        // Skip creating input fields for button properties
        if (config.type === 'button') {
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
                        
                        input = select;
                    } else {
                        // Create checkboxes for desktop or few options
                        config.options.forEach(option => {
                            const checkboxWrapper = document.createElement('div');
                            checkboxWrapper.className = 'checkbox-wrapper';
                            
                            const checkbox = document.createElement('input');
                            checkbox.type = 'checkbox';
                            checkbox.id = `${key}-${option}`;
                            checkbox.name = key;
                            checkbox.value = option;
                            checkbox.dataset.type = 'multi_select';
                            checkbox.dataset.propName = key;
                            
                            const checkboxLabel = document.createElement('label');
                            checkboxLabel.htmlFor = `${key}-${option}`;
                            checkboxLabel.textContent = option;
                            
                            checkboxWrapper.appendChild(checkbox);
                            checkboxWrapper.appendChild(checkboxLabel);
                            checkboxContainer.appendChild(checkboxWrapper);
                        });
                        
                        input = checkboxContainer;
                    }
                } else {
                    // If no options, use a text input with placeholder for tag entry
                    input = document.createElement('input');
                    input.type = 'text';
                    input.id = key;
                    input.name = key;
                    input.placeholder = 'Enter tags separated by commas';
                    input.dataset.type = 'multi_select_text';
                    input.dataset.propName = key;
                }
                break;
                
            case 'select':
                // Create dropdown for select
                input = document.createElement('select');
                input.id = key;
                input.name = key;
                input.dataset.type = 'select';
                input.dataset.propName = key;
                
                // Add empty option
                const emptyOption = document.createElement('option');
                emptyOption.value = '';
                emptyOption.textContent = '-- Select an option --';
                input.appendChild(emptyOption);
                
                // Add options
                if (config.options) {
                    config.options.forEach(option => {
                        const optionElement = document.createElement('option');
                        optionElement.value = option;
                        optionElement.textContent = option;
                        input.appendChild(optionElement);
                    });
                }
                break;
                
            case 'date':
                // Date input
                input = document.createElement('input');
                input.type = 'date';
                input.id = key;
                input.name = key;
                input.dataset.type = 'date';
                input.dataset.propName = key;
                break;
                
            case 'checkbox':
                // Checkbox input
                const checkboxDiv = document.createElement('div');
                checkboxDiv.className = 'checkbox-single';
                
                const checkbox = document.createElement('input');
                checkbox.type = 'checkbox';
                checkbox.id = key;
                checkbox.name = key;
                checkbox.dataset.type = 'checkbox';
                checkbox.dataset.propName = key;
                
                const checkboxLabel = document.createElement('label');
                checkboxLabel.htmlFor = key;
                checkboxLabel.textContent = 'Yes';
                
                checkboxDiv.appendChild(checkbox);
                checkboxDiv.appendChild(checkboxLabel);
                
                input = checkboxDiv;
                break;
                
            case 'number':
                // Number input
                input = document.createElement('input');
                input.type = 'number';
                input.id = key;
                input.name = key;
                input.dataset.type = 'number';
                input.dataset.propName = key;
                break;
                
            case 'url':
                // URL input
                input = document.createElement('input');
                input.type = 'url';
                input.id = key;
                input.name = key;
                input.placeholder = 'https://';
                input.dataset.type = 'url';
                input.dataset.propName = key;
                break;
                
            case 'email':
                // Email input
                input = document.createElement('input');
                input.type = 'email';
                input.id = key;
                input.name = key;
                input.dataset.type = 'email';
                input.dataset.propName = key;
                break;
                
            case 'phone_number':
                // Phone input
                input = document.createElement('input');
                input.type = 'tel';
                input.id = key;
                input.name = key;
                input.dataset.type = 'phone';
                input.dataset.propName = key;
                break;
                
            default:
                // Default to text input for unrecognized types
                input = document.createElement('input');
                input.type = 'text';
                input.id = key;
                input.name = key;
                input.dataset.type = 'text';
                input.dataset.propName = key;
                break;
        }
        
        // Add input to form group
        formGroup.appendChild(label);
        
        // If input is not a DOM element (like for checkbox groups), handle differently
        if (input instanceof HTMLElement) {
            formGroup.appendChild(input);
        } else {
            formGroup.appendChild(input);
        }
        
        // Add form group to container
        container.appendChild(formGroup);
    }
}

// Convert form data to the format expected by Notion API
function convertToNotionProperties(formData) {
    const result = {};
    
    // Process each form field
    for (const [key, value] of formData.entries()) {
        // Skip empty fields
        if (!value && value !== false) continue;
        
        // Get property type from dataset
        const input = document.querySelector(`[name="${key}"]`);
        if (!input) continue;
        
        const type = input.dataset.type;
        const propName = input.dataset.propName || key;
        
        // Process based on property type
        switch (type) {
            case 'multi_select':
                // Handle multi-select from select element
                if (input.tagName === 'SELECT' && input.multiple) {
                    const selected = Array.from(input.selectedOptions).map(opt => opt.value);
                    if (selected.length > 0) {
                        if (!result[propName]) result[propName] = [];
                        result[propName] = selected;
                    }
                }
                // For checkboxes, we need to collect all with the same name
                else if (input.type === 'checkbox') {
                    const checkboxes = document.querySelectorAll(`input[name="${key}"]:checked`);
                    if (checkboxes.length > 0) {
                        if (!result[propName]) result[propName] = [];
                        checkboxes.forEach(cb => {
                            result[propName].push(cb.value);
                        });
                    }
                }
                break;
                
            case 'multi_select_text':
                // Handle multi-select from text input (comma-separated)
                if (value) {
                    const tags = value.split(',').map(tag => tag.trim()).filter(tag => tag);
                    if (tags.length > 0) {
                        result[propName] = tags;
                    }
                }
                break;
                
            case 'select':
                // Handle select dropdown
                if (value) {
                    result[propName] = value;
                }
                break;
                
            case 'date':
                // Handle date input
                if (value) {
                    result[propName] = formatDateForNotion(value);
                }
                break;
                
            case 'checkbox':
                // Handle checkbox (convert to boolean)
                result[propName] = input.checked;
                break;
                
            case 'number':
                // Handle number input (parse to number)
                if (value) {
                    result[propName] = parseFloat(value);
                }
                break;
                
            default:
                // Default handling for text and other fields
                if (value) {
                    result[propName] = value;
                }
                break;
        }
    }
    
    return result;
}

// Format date string for Notion API
function formatDateForNotion(dateStr) {
    // Match YYYY-MM-DD format for Notion API
    return dateStr;
}

// Handle form submission
async function handleSubmit(event) {
    event.preventDefault();
    
    // Don't allow multiple simultaneous submissions
    if (isSubmitting) return;
    
    try {
        isSubmitting = true;
        
        // Show loading state
        const submitButton = document.getElementById('submitBtn');
        const originalButtonText = submitButton.textContent;
        submitButton.innerHTML = '<span class="loading-spinner"></span> Submitting...';
        submitButton.classList.add('loading');
        
        // Get title
        const title = document.getElementById('taskTitle').value.trim();
        if (!title) {
            throw new Error('Title is required');
        }
        
        // Get form data
        const formData = new FormData(event.target);
        
        // Convert to Notion-friendly format
        const properties = convertToNotionProperties(formData);
        
        // Create task data
        const taskData = {
            title: title,
            properties: properties
        };
        
        console.log(`Creating ${currentDbType} with:`, taskData);
        
        // Send to server
        const response = await fetch(`/notion/mini-app/api/tasks?db_type=${currentDbType}`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify(taskData)
        });
        
        const data = await response.json();
        
        if (!response.ok) {
            throw new Error(data.message || `Server error: ${response.status}`);
        }
        
        console.log('Item created successfully', data);
        
        // Show success message
        showPopup('Success', `New ${currentDbType === 'tasks' ? 'task' : 'note'} created successfully!`);
        
        // Clear the form
        document.getElementById('taskForm').reset();
        
        // If in Telegram WebApp, close after success
        if (window.Telegram && window.Telegram.WebApp) {
            // Delay to ensure user sees success message
            setTimeout(() => {
                window.Telegram.WebApp.close();
            }, 1500);
        }
    } catch (error) {
        console.error('Error creating item:', error);
        showError(error.message || 'Failed to create item');
    } finally {
        // Reset submit button
        const submitButton = document.getElementById('submitBtn');
        submitButton.textContent = 'Create Item';
        submitButton.classList.remove('loading');
        isSubmitting = false;
    }
}

// Show error message
function showError(message) {
    const errorContainer = document.getElementById('error-container');
    errorContainer.textContent = message;
    errorContainer.style.display = 'block';
    
    // Hide after 5 seconds
    setTimeout(() => {
        errorContainer.style.display = 'none';
    }, 5000);
}

// Show popup message
function showPopup(title, message) {
    // Check if we have native Telegram popup
    if (window.Telegram && window.Telegram.WebApp) {
        window.Telegram.WebApp.showPopup({
            title: title,
            message: message,
            buttons: [{ type: 'ok' }]
        });
        return;
    }
    
    // Fallback for browser
    alert(`${title}: ${message}`);
} 