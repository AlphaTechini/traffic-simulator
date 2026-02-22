<script lang="ts">
  import { onMount } from 'svelte';
  
  let urls: Array<{id: string, url: string, config: any}> = [];
  let newUrl = '';
  let isDragging = false;
  
  onMount(() => {
    // Load saved scenario if exists
    const saved = localStorage.getItem('current-scenario');
    if (saved) {
      urls = JSON.parse(saved);
    }
  });
  
  function addUrl() {
    if (!newUrl.trim()) return;
    
    urls.push({
      id: crypto.randomUUID(),
      url: newUrl.trim(),
      config: {
        method: 'POST',
        content_type: 'application/json',
        payload: '{}',
        weight: 1
      }
    });
    
    newUrl = '';
    saveScenario();
  }
  
  function removeUrl(id: string) {
    urls = urls.filter(u => u.id !== id);
    saveScenario();
  }
  
  function updateConfig(id: string, field: string, value: any) {
    const url = urls.find(u => u.id === id);
    if (url) {
      url.config[field] = value;
      saveScenario();
    }
  }
  
  function saveScenario() {
    localStorage.setItem('current-scenario', JSON.stringify(urls));
  }
  
  function handleDrop(e: DragEvent) {
    e.preventDefault();
    isDragging = false;
    
    const text = e.dataTransfer?.getData('text');
    if (text && isValidUrl(text)) {
      urls.push({
        id: crypto.randomUUID(),
        url: text,
        config: {
          method: 'GET',
          content_type: 'application/json',
          payload: '{}',
          weight: 1
        }
      });
      saveScenario();
    }
  }
  
  function isValidUrl(string: string) {
    try {
      new URL(string);
      return true;
    } catch (_) {
      return false;
    }
  }
  
  function startSimulation() {
    // TODO: Call API to start simulation
    console.log('Starting simulation with:', urls);
    alert('Simulation started! (API integration pending)');
  }
</script>

<svelte:head>
  <title>Traffic Simulator - Configure Load Test</title>
  <meta name="description" content="Configure and run distributed load tests with up to 5M concurrent users" />
</svelte:head>

<div class="min-h-screen">
  <!-- Header -->
  <header class="bg-white shadow-sm border-b">
    <div class="max-w-7xl mx-auto px-4 py-4 flex justify-between items-center">
      <div class="flex items-center gap-3">
        <span class="text-3xl">🚀</span>
        <h1 class="text-2xl font-bold text-gray-900">Traffic Simulator</h1>
      </div>
      <nav class="flex gap-4">
        <a href="/scenarios" class="text-gray-600 hover:text-primary">Scenarios</a>
        <a href="/results" class="text-gray-600 hover:text-primary">Results</a>
        <a href="/settings" class="text-gray-600 hover:text-primary">Settings</a>
      </nav>
    </div>
  </header>

  <!-- Main Content -->
  <main class="max-w-7xl mx-auto px-4 py-8">
    <!-- URL Dropzone -->
    <section class="mb-8">
      <h2 class="text-xl font-semibold mb-4">Target Endpoints</h2>
      
      <div
        class="border-2 border-dashed rounded-lg p-8 text-center transition-colors"
        class:blue-200={isDragging}
        class:border-blue-400={isDragging}
        role="button"
        tabindex="0"
        aria-label="Drag and drop URLs here or paste below"
        ondragover={(e) => { e.preventDefault(); isDragging = true; }}
        ondragleave={() => isDragging = false}
        ondrop={handleDrop}
      >
        <div class="text-gray-500 mb-4">
          <svg class="mx-auto h-12 w-12" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M15 13l-3-3m0 0l-3 3m3-3v12" />
          </svg>
          <p class="mt-2">Drag and drop URLs here, or paste below</p>
        </div>
        
        <div class="flex gap-2 max-w-2xl mx-auto">
          <input
            type="url"
            bind:value={newUrl}
            placeholder="https://api.example.com/graphql"
            class="input flex-1"
          />
          <button class="btn-primary" onclick={addUrl}>
            Add Endpoint
          </button>
        </div>
      </div>
    </section>

    <!-- Configured Endpoints -->
    {#if urls.length > 0}
      <section class="mb-8">
        <h2 class="text-xl font-semibold mb-4">Endpoint Configuration ({urls.length})</h2>
        
        <div class="space-y-4">
          {#each urls as url (url.id)}
            <div class="card">
              <div class="flex justify-between items-start mb-4">
                <div class="flex-1">
                  <input
                    type="url"
                    bind:value={url.url}
                    class="input font-mono text-sm"
                    onchange={() => saveScenario()}
                  />
                </div>
                <button
                  class="text-red-600 hover:text-red-800 ml-4"
                  onclick={() => removeUrl(url.id)}
                  aria-label="Remove endpoint"
                  title="Remove endpoint"
                >
                  <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                  </svg>
                </button>
              </div>
              
              <div class="grid grid-cols-2 md:grid-cols-4 gap-4">
                <div>
                  <label class="block text-sm font-medium text-gray-700 mb-1" id="method-label-{url.id}">Method</label>
                  <select
                    bind:value={url.config.method}
                    class="input"
                    aria-labelledby="method-label-{url.id}"
                    onchange={() => saveScenario()}
                  >
                    <option>GET</option>
                    <option>POST</option>
                    <option>PUT</option>
                    <option>DELETE</option>
                    <option>PATCH</option>
                  </select>
                </div>
                
                <div>
                  <label class="block text-sm font-medium text-gray-700 mb-1" id="content-type-label-{url.id}">Content Type</label>
                  <select
                    bind:value={url.config.content_type}
                    class="input"
                    aria-labelledby="content-type-label-{url.id}"
                    onchange={() => saveScenario()}
                  >
                    <option>application/json</option>
                    <option>application/graphql</option>
                    <option>application/x-www-form-urlencoded</option>
                    <option>text/plain</option>
                  </select>
                </div>
                
                <div>
                  <label class="block text-sm font-medium text-gray-700 mb-1" id="weight-label-{url.id}">Weight</label>
                  <input
                    type="number"
                    bind:value={url.config.weight}
                    min="1"
                    class="input"
                    aria-labelledby="weight-label-{url.id}"
                    onchange={() => saveScenario()}
                  />
                </div>
                
                <div>
                  <label class="block text-sm font-medium text-gray-700 mb-1" id="request-type-label-{url.id}">Request Type</label>
                  <select
                    bind:value={url.config.request_type}
                    class="input"
                    aria-labelledby="request-type-label-{url.id}"
                    onchange={() => saveScenario()}
                  >
                    <option>rest</option>
                    <option>graphql</option>
                    <option>websocket</option>
                  </select>
                </div>
              </div>
              
              <div class="mt-4">
                <label class="block text-sm font-medium text-gray-700 mb-1" id="payload-label-{url.id}">
                  JSON Payload (use {{variable}} syntax)
                </label>
                <textarea
                  bind:value={url.config.payload}
                  class="input font-mono text-sm h-32"
                  aria-labelledby="payload-label-{url.id}"
                  onchange={() => saveScenario()}
                ></textarea>
                <div class="mt-2 text-xs text-gray-500">
                  Available variables: {{uuid}}, {{email}}, {{timestamp}}, {{increment}}, {{random_string}}
                </div>
              </div>
            </div>
          {/each}
        </div>
      </section>
    {/if}

    <!-- Simulation Controls -->
    <section class="card">
      <h2 class="text-xl font-semibold mb-6">Simulation Settings</h2>
      
      <div class="grid grid-cols-1 md:grid-cols-3 gap-6 mb-6">
        <div>
          <label class="block text-sm font-medium text-gray-700 mb-1" for="total-users">Total Users</label>
          <input type="number" id="total-users" class="input" value="10000" min="1" />
        </div>
        
        <div>
          <label class="block text-sm font-medium text-gray-700 mb-1" for="duration">Duration</label>
          <input type="text" id="duration" class="input" value="30m" placeholder="e.g., 30m, 1h" />
        </div>
        
        <div>
          <label class="block text-sm font-medium text-gray-700 mb-1" for="traffic-pattern">Traffic Pattern</label>
          <select id="traffic-pattern" class="input">
            <option>Constant</option>
            <option>Ramp Up</option>
            <option>Step</option>
            <option>Wave</option>
            <option>Burst</option>
          </select>
        </div>
      </div>
      
      <div class="flex gap-4">
        <button class="btn-primary text-lg px-8" onclick={startSimulation}>
          🚀 Start Simulation
        </button>
        <button class="btn-secondary">
          💾 Save Scenario
        </button>
        <button class="btn-secondary">
          📊 Preview Load
        </button>
      </div>
    </section>
  </main>
</div>

<style>
  :global(.blue-200) {
    background-color: #bfdbfe;
    border-color: #60a5fa;
  }
</style>
