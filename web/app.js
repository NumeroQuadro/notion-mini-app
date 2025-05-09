const tg = window.Telegram?.WebApp;
if(tg) tg.expand();

// Cache and state
let schemaCache = {};
let submitting = false;
let currentDbType = "tasks"; // Track current database type

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
    
    // Update submit button text based on current db type
    const submitBtn = document.getElementById('submitBtn');
    if (submitBtn) {
      submitBtn.textContent = `Create ${currentDbType === 'tasks' ? 'Task' : 'Note'}`;
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
      
      // Skip the "Description" property since we already have a dedicated field for it
      if(key.toLowerCase() === 'description') continue;
      
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
    
    // Get description field
    const description = form.get('description');
    if(description && description.trim()) {
      // Add description as a property
      props["Description"] = description.trim();
    }
    
    // Process form data according to schema types
    for(const [k,v] of form){
      if(k === 'taskTitle' || k === 'description') continue;
      
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
      showMessage(`${currentDbType === 'tasks' ? 'Task' : 'Note'} created successfully!`, false);
      e.target.reset();
      
      // Close Telegram mini app if available
      if(tg) {
        setTimeout(() => tg.close(), 1500);
      }
    } catch (apiError) {
      console.error("API error:", apiError);
      // Show a more user-friendly message for API errors
      if (apiError.message && apiError.message.includes("validation")) {
        showMessage("The description might be too long or contain unsupported formatting. Try simplifying it.", true);
      } else {
        showMessage('Error: ' + apiError.message, true);
      }
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

// Setup tab switching
function setupTabs() {
  document.querySelectorAll('.tab').forEach(tab => {
    tab.addEventListener('click', async function() {
      if (submitting) return; // Don't switch tabs during submission
      
      // Update active tab
      document.querySelectorAll('.tab').forEach(t => t.classList.remove('active'));
      this.classList.add('active');
      
      // Get and store database type
      currentDbType = this.getAttribute('data-db-type');
      console.log(`Switched to ${currentDbType} database`);
      
      // Rebuild form for this database type
      await buildForm();
    });
  });
}

// Check database availability from config
async function checkDatabaseAvailability() {
  try {
    const response = await fetch('/notion/mini-app/api/config');
    if (!response.ok) return;
    
    const config = await response.json();
    
    // Hide notes tab if not available
    if (config.HAS_NOTES_DB !== "true") {
      const notesTab = document.getElementById('notesTab');
      if (notesTab) notesTab.style.display = 'none';
    }
    
    // Hide tasks tab if not available
    if (config.HAS_TASKS_DB !== "true") {
      const tasksTab = document.getElementById('tasksTab');
      if (tasksTab) tasksTab.style.display = 'none';
      
      // If tasks not available but notes is, switch to notes
      if (config.HAS_NOTES_DB === "true") {
        currentDbType = "notes";
        const notesTab = document.getElementById('notesTab');
        if (notesTab) {
          notesTab.classList.add('active');
          document.getElementById('tasksTab')?.classList.remove('active');
        }
      }
    }
  } catch (error) {
    console.error("Error checking database availability:", error);
    showWarning("Could not check database availability");
  }
}

document.addEventListener('DOMContentLoaded', async () => {
  // Setup tab switching
  setupTabs();
  
  // Check which databases are available
  await checkDatabaseAvailability();
  
  // Build the initial form
  await buildForm();
  
  // Setup form submission
  document.getElementById('taskForm').addEventListener('submit', handleSubmit);
});