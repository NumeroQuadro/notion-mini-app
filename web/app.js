// public/mini-app.js — concise Notion mini-app script
const tg = window.Telegram?.WebApp;
if(tg) tg.expand();

// Cache and state
let schemaCache = {};
let submitting = false;

// Renderers for different property types
const renderers = {
  title:        () => null,
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
  // Explicitly ignore button property type
  button:       () => null,
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
  cfg.options.forEach(o=> sel.append(create('option',{value:o},o)));
  return sel;
}

function multiSelectField(key,cfg){
  // large lists on mobile get a multi-select dropdown
  if(cfg.options.length>5 && /Mobi/i.test(navigator.userAgent)){
    const sel=create('select',{id:key,name:key,multiple:true});
    cfg.options.forEach(o=> sel.append(create('option',{value:o},o)));
    return sel;
  }
  // else checkbox list
  return cfg.options.map(o=>
    create('label',{},
      create('input',{type:'checkbox',name:key,value:o}), ' '+o
    )
  );
}

async function fetchSchema(){
  try {
    if(!schemaCache.tasks){
      const r=await fetch('/notion/mini-app/api/properties?db_type=tasks');
      if(!r.ok) {
        // Try to get properties even from an error response
        const data = await r.json();
        if (data && data.properties) {
          schemaCache.tasks = data.properties;
          
          // Show warning if we have an error message but still got properties
          if(data.error && data.error.includes('button')) {
            showWarning("Some button properties in the database are skipped");
          }
        } else {
          throw Error(data?.error || r.statusText);
        }
      } else {
        schemaCache.tasks = await r.json();
      }
    }
    return schemaCache.tasks;
  } catch(err) {
    console.error("Error fetching schema:", err);
    // Return empty schema as fallback
    return {};
  }
}

// Show a warning message that doesn't block usage
function showWarning(message) {
  const container = document.getElementById('propertiesContainer');
  if (!container) return;
  
  const warningDiv = create('div', 
    {className: 'warning-message'}, 
    message
  );
  
  // Insert warning before the first child
  container.insertBefore(warningDiv, container.firstChild);
  console.warn(message);
}

async function buildForm(){
  try {
    const schema = await fetchSchema();
    const container = document.getElementById('propertiesContainer');
    container.innerHTML = '';
    
    // If schema is empty, show warning but still allow form submission
    if(Object.keys(schema).length === 0) {
      showWarning("No database properties found. Only the title field will be available.");
    }
    
    for(const [key,cfg] of Object.entries(schema)){
      // Skip title and button properties
      if(cfg.type === 'title' || cfg.type === 'button') continue;
      
      // Skip properties with "button" in their name
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
    showWarning("Error loading database properties. You can still submit with just a title.");
  }
}

async function handleSubmit(e){
  e.preventDefault(); if(submitting) return;
  submitting = true;
  
  const btn = e.target.querySelector('#submitBtn');
  const originalText = btn.textContent;
  btn.disabled = true; 
  btn.innerHTML = '<span class="loading-spinner"></span> Submitting...';
  
  try{
    const schema = await fetchSchema();
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
      
      const type = schema[k]?.type;
      
      // Skip button properties
      if(type === 'button' || (k.toLowerCase && k.toLowerCase().includes('button'))) {
        continue;
      }
      
      if(type === 'checkbox') {
        props[k] = e.target[k].checked;
      } else if(type === 'multi_select') {
        const values = form.getAll(k);
        if(values.length > 0) {
          props[k] = values;
        }
      } else if(v) {
        props[k] = v;
      }
    }
    
    const payload = {title: title.trim(), properties: props};
    const res = await fetch('/notion/mini-app/api/tasks?db_type=tasks', {
      method: 'POST',
      headers: {'Content-Type': 'application/json'},
      body: JSON.stringify(payload)
    });
    
    if(!res.ok) {
      const errorData = await res.json();
      throw Error(errorData?.message || errorData?.error || 'Server error');
    }
    
    // Success!
    alert('Task created successfully!');
    e.target.reset();
    
    // Close Telegram mini app if available
    if(tg) {
      setTimeout(() => tg.close(), 1000);
    }
  } catch(err) {
    // Handle button property errors with a more specific message
    if(err.message && (err.message.includes('button') || err.message.includes('unsupported property type'))) {
      alert('Error: The database contains button properties that are not supported. Your task was not saved.');
    } else {
      alert('Error: ' + err.message);
    }
  } finally {
    btn.disabled = false;
    btn.textContent = originalText;
    submitting = false;
  }
}

document.addEventListener('DOMContentLoaded', () => {
  buildForm();
  document.getElementById('taskForm').addEventListener('submit', handleSubmit);
});