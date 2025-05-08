// Initialize Telegram WebApp
const tg = window.Telegram.WebApp;
tg.expand();

// Get database properties from the backend
async function getDatabaseProperties() {
    try {
        const response = await fetch('/api/properties');
        const properties = await response.json();
        return properties;
    } catch (error) {
        console.error('Error fetching properties:', error);
        return {};
    }
}

// Create form fields based on Notion properties
async function createPropertyFields() {
    const properties = await getDatabaseProperties();
    const container = document.getElementById('propertiesContainer');

    for (const [key, config] of Object.entries(properties)) {
        const formGroup = document.createElement('div');
        formGroup.className = 'form-group';

        const label = document.createElement('label');
        label.htmlFor = key;
        label.textContent = key;

        let input;
        switch (config.type) {
            case 'select':
                input = document.createElement('select');
                config.options.forEach(option => {
                    const optionElement = document.createElement('option');
                    optionElement.value = option;
                    optionElement.textContent = option;
                    input.appendChild(optionElement);
                });
                break;
            case 'date':
                input = document.createElement('input');
                input.type = 'date';
                break;
            default:
                input = document.createElement('input');
                input.type = 'text';
        }

        input.id = key;
        input.name = key;
        input.required = config.required || false;

        formGroup.appendChild(label);
        formGroup.appendChild(input);
        container.appendChild(formGroup);
    }
}

// Handle form submission
async function handleSubmit(event) {
    event.preventDefault();

    const formData = new FormData(event.target);
    const taskData = {
        title: formData.get('taskTitle'),
        properties: {}
    };

    // Collect all property values
    for (const [key, value] of formData.entries()) {
        if (key !== 'taskTitle') {
            taskData.properties[key] = value;
        }
    }

    try {
        const response = await fetch('/api/tasks', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify(taskData),
        });

        if (response.ok) {
            tg.showPopup({
                title: 'Success',
                message: 'Task created successfully!',
                buttons: [{ type: 'ok' }]
            });
            tg.close();
        } else {
            throw new Error('Failed to create task');
        }
    } catch (error) {
        console.error('Error creating task:', error);
        tg.showPopup({
            title: 'Error',
            message: 'Failed to create task. Please try again.',
            buttons: [{ type: 'ok' }]
        });
    }
}

// Initialize the app
document.addEventListener('DOMContentLoaded', () => {
    createPropertyFields();
    document.getElementById('taskForm').addEventListener('submit', handleSubmit);
}); 