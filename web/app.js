const tg = window.Telegram?.WebApp;
if(tg) tg.expand();

// Cache and state
let schemaCache = {};
let submitting = false;
let currentDbType = "tasks"; // Track current database type
let currentSection = "home"; // Track current section (home, form, recent-tasks)

// Renderers for different property types
const renderers = {
  title:        () => null, // Skip title as it's handled separately
  rich_text:    (k,c) => create('input',{type:'text',id:k,name:k,required:c.required}),
  text:         (k,c) => create('input',{type:'text',id:k,name:k,required:c.required}),
  date:         (k,c) => create('input',{type:'date',id:k,name:k,required:c.required}),
  select:       (k,c) => selectField(k,c),
  multi_select: (k,c) => multiSelectField(k,c),
  checkbox:     (k,c) => create('label',{},
                 create('input',{type:'checkbox',id:k,name:k}), ' ' + k),
  number:       (k,c) => create('input',{type:'number',id:k,name:k,required:c.required}),
  url:          (k,c) => create('input',{type:'url',id:k,name:k,placeholder:'https://',required:c.required}),
  email:        (k,c) => create('input',{type:'email',id:k,name:k,required:c.required}),
  phone_number: (k,c) => create('input',{type:'tel',id:k,name:k,required:c.required}),
  // Explicitly skip button property type
  button:       () => null,
  unsupported:  () => null
};

// Helper to create elements
function create(tag,attrs={},...kids){
  const e=document.createElement(tag);
  Object.assign(e,attrs);
  kids.flat().forEach(c=>e.append(typeof c=='string'?document.createTextNode(c):c));
  return e;
}

function selectField(key, cfg) {
  const sel = create('select',{id:key,name:key,required:cfg.required}, create('option',{value:''},'--Select--'));
  (cfg.options || []).forEach(o=> sel.append(create('option',{value:o},o)));
  return sel;
}

function multiSelectField(key,cfg){
  // If no options, return a text input as fallback
  if (!cfg.options || cfg.options.length === 0) {
    return create('input',{type:'text',id:key,name:key,placeholder:'Comma-separated values',required:cfg.required});
  }
  
  // Special case for Mood - always use scrollable multi-select
  if(key === 'Mood') {
    const wrapper = create('div', {className: 'scrollable-multiselect'});
    const checkboxes = document.createElement('div');
    checkboxes.className = 'checkbox-container';
    
    cfg.options.forEach(o => {
      const label = document.createElement('label');
      label.className = 'checkbox-label';
      
      const checkbox = document.createElement('input');
      checkbox.type = 'checkbox';
      checkbox.name = key;
      checkbox.value = o;
      
      label.appendChild(checkbox);
      label.appendChild(document.createTextNode(' ' + o));
      checkboxes.appendChild(label);
    });
    
    wrapper.appendChild(checkboxes);
    return wrapper;
  }
  
  // Large lists on mobile get a multi-select dropdown
  if(cfg.options.length>5 && /Mobi/i.test(navigator.userAgent)){
    const sel=create('select',{id:key,name:key,multiple:true});
    cfg.options.forEach(o=> sel.append(create('option',{value:o},o)));
    return sel;
  }
  
  // Else checkbox list
  return cfg.options.map(o=>
    create('label',{className:'checkbox-label'},
      create('input',{type:'checkbox',name:key,value:o}), ' '+o
    )
  );
}

async function fetchSchema(dbType = "tasks"){
  try {
    if(!schemaCache[dbType]){
      const r=await fetch(`/notion/mini-app/api/properties?db_type=${dbType}`);
      const data = await r.json();
      
      if (data && data.properties) {
        schemaCache[dbType] = data.properties;
        
        // Show warning if we have an error message but still got properties
        if(data.error && data.error.includes('button')) {
          showWarning("Some button properties in the database are skipped");
        }
      } else if (data && Object.keys(data).length > 0) {
        // Regular response format
        schemaCache[dbType] = data;
        } else {
        console.warn("Schema response empty or invalid:", data);
        // Return empty schema as fallback
        schemaCache[dbType] = {};
      }
    }
    return schemaCache[dbType];
  } catch(err) {
    console.error("Error fetching schema:", err);
    showWarning(`Error loading database schema: ${err.message}`);
    // Return empty schema as fallback
    return {};
  }
}

// Show a warning message that doesn't block usage
function showWarning(message) {
  const container = document.getElementById('propertiesContainer');
  if (!container) return;
  
  // Remove any existing warnings first
  const existingWarnings = container.querySelectorAll('.warning-message');
  existingWarnings.forEach(warning => warning.remove());
  
  const warningDiv = create('div', 
    {className: 'warning-message'}, 
    message
  );
  
  // Insert warning before the first child
  container.insertBefore(warningDiv, container.firstChild);
  console.warn(message);
}

// Show message in the UI - can be used for both errors and success messages
function showMessage(message, isError = true) {
  const errorContainer = document.getElementById('error-container');
  if (!errorContainer) return;
  
  // Format API errors to be more user-friendly
  if (isError && message.includes("Notion API error")) {
    message = "There was an error saving to Notion. Please try again or contact support.";
  }
  
  errorContainer.textContent = message;
  errorContainer.style.display = 'block';
  
  if (isError) {
    errorContainer.classList.add('error-message');
    errorContainer.classList.remove('success-message');
  } else {
    errorContainer.classList.add('success-message');
    errorContainer.classList.remove('error-message');
  }
  
  // Auto-hide after 5 seconds
  setTimeout(() => {
    errorContainer.style.display = 'none';
  }, 5000);
}

async function buildForm(){
  try {
    const schema = await fetchSchema(currentDbType);
    const container = document.getElementById('propertiesContainer');
    container.innerHTML = '';

    // Clear error container
    const errorContainer = document.getElementById('error-container');
    if (errorContainer) errorContainer.style.display = 'none';
    
    // Update form title and submit button text based on current db type
    const formTitle = document.getElementById('formTitle');
    if (formTitle) {
      if (currentDbType === 'tasks') {
        formTitle.textContent = 'Create New Task';
      } else if (currentDbType === 'notes') {
        formTitle.textContent = 'Create New Note';
      } else if (currentDbType === 'journal') {
        formTitle.textContent = 'Create Journal Entry';
      } else {
        formTitle.textContent = 'Create New Item';
      }
    }
    
    // Update submit button text
    const submitBtn = document.getElementById('submitBtn');
    if (submitBtn) {
      if (currentDbType === 'tasks') {
        submitBtn.textContent = 'Create Task';
      } else if (currentDbType === 'notes') {
        submitBtn.textContent = 'Create Note';
      } else if (currentDbType === 'journal') {
        submitBtn.textContent = 'Save Entry';
      } else {
        submitBtn.textContent = 'Create Item';
      }
    }
    
    // If schema is empty, show warning but still allow form submission
    if(!schema || Object.keys(schema).length === 0) {
      showWarning(`No ${currentDbType} database properties found. Only the title field will be available.`);
      return;
    }
    
    for(const [key,cfg] of Object.entries(schema)){
      // Skip title, button, and unsupported properties
      if(cfg.type === 'title' || cfg.type === 'button' || cfg.type === 'unsupported') continue;
      
      // Skip properties with "button" in their name to avoid potential issues
      if(key.toLowerCase().includes('button')) continue;
      
      // Get renderer for this property type or default to text input
      const render = renderers[cfg.type] || renderers.text;
      const field = render(key, cfg);
      if(!field) continue;
      
      const wrapper = create('div', {className: 'form-group'},
        create('label', {htmlFor: key}, key + (cfg.required ? ' *' : '')),
        field
      );
      container.append(wrapper);
    }
  } catch(err) {
    console.error("Error building form:", err);
    showWarning(`Error loading ${currentDbType} database properties. You can still submit with just a title.`);
  }
}

async function handleSubmit(e){
  e.preventDefault(); 
  if(submitting) return;
  
  submitting = true;
  
  // Clear any previous messages
  const errorContainer = document.getElementById('error-container');
  if (errorContainer) errorContainer.style.display = 'none';
  
  const btn = e.target.querySelector('#submitBtn');
  const originalText = btn.textContent;
  btn.disabled = true; 
  btn.innerHTML = '<span class="loading-spinner"></span> Submitting...';
  
  try{
    const schema = await fetchSchema(currentDbType);
    const form = new FormData(e.target);
    const props = {};
    
    // Get title field
    const title = form.get('taskTitle');
    if(!title || !title.trim()) {
      throw Error("Title is required");
    }
    
    // Process form data according to schema types
    for(const [k,v] of form){
      if(k === 'taskTitle') continue;
      
      // If no schema or property not in schema, skip
      if(!schema || !schema[k]) continue;
      
      const type = schema[k]?.type;
      
      // Skip button properties and properties with button in name
      if(type === 'button' || type === 'unsupported' || 
         (k.toLowerCase && k.toLowerCase().includes('button'))) {
        continue;
      }
      
      // Handle different property types
      switch(type) {
        case 'checkbox':
          props[k] = e.target[k].checked;
                break;
        case 'multi_select':
          const values = form.getAll(k);
          if(values.length > 0) {
            // If it's a text input (no options in schema), split by comma
            if(values.length === 1 && values[0].includes(',')) {
              props[k] = values[0].split(',').map(v => v.trim()).filter(v => v);
            } else {
              props[k] = values;
            }
                }
                break;
            case 'number':
          if(v) props[k] = parseFloat(v);
                break;
            default:
          if(v) props[k] = v;
      }
    }
    
    const payload = {title: title.trim(), properties: props};
    console.log("Submitting data:", payload);
    
    try {
      const res = await fetch(`/notion/mini-app/api/tasks?db_type=${currentDbType}`, {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify(payload)
      });
      
      const responseData = await res.json();
      
      if(!res.ok) {
        console.error("Server error:", responseData);
        throw Error(responseData?.message || responseData?.error || 'Server error');
      }
      
      // Success!
      showMessage(`${currentDbType === 'tasks' ? 'Task' : currentDbType === 'notes' ? 'Note' : 'Journal entry'} created successfully!`, false);
      e.target.reset();
    } catch (apiError) {
      console.error("API error:", apiError);
      showMessage('Error saving to Notion. Please try again.', true);
    }
  } catch(err) {
    console.error("Form error:", err);
    // Handle button property errors with a more specific message
    if(err.message && (err.message.includes('button') || err.message.includes('unsupported property type'))) {
      showMessage(`Error: The database contains button properties that are not supported. Your item was not saved.`, true);
    } else {
      showMessage('Error: ' + err.message, true);
    }
  } finally {
    btn.disabled = false;
    btn.textContent = originalText;
    submitting = false;
  }
}

// Setup tile navigation
function setupTileNavigation() {
  // Add click handler to all tiles
  document.querySelectorAll('.tile').forEach(tile => {
    tile.addEventListener('click', () => {
      if (submitting) return; // Don't navigate during submission
      
      if (tile.dataset.action === 'recent-tasks') {
        navigateTo('recent-tasks');
        return;
      }
      
      // Handle database tiles
      if (tile.dataset.dbType) {
        currentDbType = tile.dataset.dbType;
        navigateTo('form');
      }
    });
  });
  
  // Setup back button
  document.querySelector('.back-btn')?.addEventListener('click', () => {
    navigateTo('home');
  });
}

// Navigate between sections
function navigateTo(section) {
  currentSection = section;
  
  // Hide all sections
  document.getElementById('home-screen').style.display = 'none';
  document.getElementById('formSection').style.display = 'none';
  document.getElementById('recentTasksSection').style.display = 'none';
  
  // Show back button for non-home sections
  document.getElementById('back-button').style.display = section === 'home' ? 'none' : 'block';
  
  // Show the selected section
  switch(section) {
    case 'home':
      document.getElementById('home-screen').style.display = 'block';
      break;
    case 'form':
      document.getElementById('formSection').style.display = 'block';
      buildForm(); // Build/rebuild the form with current database type
      break;
    case 'recent-tasks':
      document.getElementById('recentTasksSection').style.display = 'block';
      loadRecentTasks();
      break;
  }
}

// Fetch and display recent tasks
async function loadRecentTasks() {
  const tasksList = document.getElementById('tasksList');
  tasksList.innerHTML = '<div class="loading-indicator">Loading recent tasks...</div>';
  
  try {
    const response = await fetch('/notion/mini-app/api/recent-tasks?db_type=tasks');
    if (!response.ok) {
      throw new Error('Failed to fetch recent tasks');
    }
    
    const tasks = await response.json();
    
    if (tasks.length === 0) {
      tasksList.innerHTML = '<div class="no-tasks">No tasks found matching criteria</div>';
            return;
        }
        
    tasksList.innerHTML = '';
    
    // Create a task list
    const taskList = document.createElement('ul');
    taskList.className = 'task-list';
    
    // Add each task to the list
    tasks.forEach(task => {
      const taskItem = document.createElement('li');
      taskItem.className = 'task-item';
      
      // Create task title with link
      const taskTitle = document.createElement('a');
      taskTitle.href = task.url;
      taskTitle.target = '_blank';
      taskTitle.textContent = task.title;
      
      // Add task properties that might be useful to display
      let taskDetails = '';
      if (task.properties.status) {
        taskDetails += `<div class="task-status">Status: ${task.properties.status}</div>`;
      }
      if (task.properties.Tags && Array.isArray(task.properties.Tags)) {
        taskDetails += `<div class="task-tags">Tags: ${task.properties.Tags.join(', ')}</div>`;
      }
      
      // Simplify date format - only show date without time and without "Created:" label
      const createdDate = new Date(task.created_at);
      const formattedDate = createdDate.toLocaleDateString();
      
      // Add checkbox for task completion
      const checkboxId = `task-complete-${task.id}`;
      
      taskItem.innerHTML = `
        <div class="task-header">
          <div class="task-checkbox">
            <input type="checkbox" id="${checkboxId}" class="task-complete-checkbox" data-task-id="${task.id}">
            <label for="${checkboxId}"></label>
          </div>
          <div class="task-title">${taskTitle.outerHTML}</div>
          <div class="task-date">${formattedDate}</div>
        </div>
        <div class="task-properties">
          ${taskDetails}
        </div>
      `;
      
      taskList.appendChild(taskItem);
    });
    
    tasksList.appendChild(taskList);
    
    // Add event listeners to checkboxes
    document.querySelectorAll('.task-complete-checkbox').forEach(checkbox => {
      checkbox.addEventListener('change', handleTaskComplete);
    });
    
  } catch (error) {
    console.error('Error loading recent tasks:', error);
    tasksList.innerHTML = `<div class="error-message">Error loading tasks: ${error.message}</div>`;
  }
}

// Handle task completion
async function handleTaskComplete(event) {
  const checkbox = event.target;
  const taskId = checkbox.dataset.taskId;
  const taskItem = checkbox.closest('.task-item');
  
  // Disable the checkbox while processing
  checkbox.disabled = true;
  
  try {
    // Show loading state
    taskItem.classList.add('updating');
    
    // Update task status to "done"
    const response = await fetch('/notion/mini-app/api/update-task-status', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json'
      },
      body: JSON.stringify({
        task_id: taskId,
        status: 'done'
      })
    });
    
    if (!response.ok) {
      const data = await response.json();
      throw new Error(data.error || 'Failed to update task status');
    }
    
    // Show success indicator
    taskItem.classList.remove('updating');
    taskItem.classList.add('completed');
    
    // Add delay before removing from list
    setTimeout(() => {
      taskItem.style.opacity = '0';
      setTimeout(() => {
        taskItem.remove();
        
        // If no tasks left, show message
        if (document.querySelectorAll('.task-item').length === 0) {
          document.getElementById('tasksList').innerHTML = '<div class="no-tasks">No tasks found matching criteria</div>';
        }
      }, 300);
    }, 1000);
    
  } catch (error) {
    console.error('Error updating task:', error);
    
    // Reset checkbox
    checkbox.checked = false;
    checkbox.disabled = false;
    taskItem.classList.remove('updating');
    
    // Show error message
    showMessage(`Error completing task: ${error.message}`, true);
  }
}

// Check database availability from config
async function checkDatabaseAvailability() {
  try {
    const response = await fetch('/notion/mini-app/api/config');
    if (!response.ok) return;
    
    const config = await response.json();
    
    // Hide database tiles if not available
    if (config.HAS_NOTES_DB !== "true") {
      const notesTile = document.querySelector('.notes-tile');
      if (notesTile) notesTile.style.display = 'none';
    }
    
    if (config.HAS_TASKS_DB !== "true") {
      const tasksTile = document.querySelector('.tasks-tile');
      if (tasksTile) tasksTile.style.display = 'none';
      
      // Hide recent tasks tile if tasks DB is not available
      const recentTasksTile = document.querySelector('.recent-tasks-tile');
      if (recentTasksTile) recentTasksTile.style.display = 'none';
    }
    
    if (config.HAS_JOURNAL_DB !== "true") {
      const journalTile = document.querySelector('.journal-tile');
      if (journalTile) journalTile.style.display = 'none';
        }
    } catch (error) {
    console.error("Error checking database availability:", error);
    showWarning("Could not check database availability");
  }
}

document.addEventListener('DOMContentLoaded', async () => {
  // Setup tile navigation
  setupTileNavigation();
  
  // Check which databases are available
  await checkDatabaseAvailability();
  
  // Setup form submission
    document.getElementById('taskForm').addEventListener('submit', handleSubmit);
  
  // Start on the home screen
  navigateTo('home');
}); 