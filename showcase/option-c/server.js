/**
 * Node.js HTTP Server for Intent Task Queue Showcase (Option C)
 *
 * IMPORTANT: This server uses UNMODIFIED compiler-generated JavaScript from:
 * ./task_queue.generated.js
 *
 * The generated file is pure compiler output and must not be hand-edited.
 * This demonstrates how Intent's compiler-generated code can be seamlessly
 * integrated into a Node.js server application with contract enforcement
 * and runtime safety guarantees.
 */

const http = require('http');
const fs = require('fs');
const path = require('path');
const vm = require('vm');

// Load the compiler-generated code using vm module to avoid process.exit.
// Class declarations in vm contexts don't automatically become sandbox properties,
// so we wrap the code to explicitly export them.
const generatedCode = fs.readFileSync(path.join(__dirname, 'task_queue.generated.js'), 'utf8');

// Create a sandbox that suppresses console output and prevents process.exit
const sandbox = {
  console: { log: () => {} },
  process: { exit: () => {} },
  Error: Error,
  Array: Array,
  Object: Object,
};

// Wrap the generated code to export the defined classes and functions.
// The generated code defines: JobStatus, Job, Worker, find_highest_priority,
// count_by_status, __intent_main. We capture them via a return object.
const wrappedCode = `(function() { ${generatedCode}\nreturn { JobStatus, Job, Worker, find_highest_priority, count_by_status }; })()`;
vm.createContext(sandbox);
const intentExports = vm.runInContext(wrappedCode, sandbox);

// Extract the classes and functions
const { JobStatus, Job, Worker, find_highest_priority, count_by_status } = intentExports;

// Demo state management
let demoState = null;

function initializeDemo() {
  const jobs = [
    new Job(1, 5),
    new Job(2, 9),
    new Job(3, 3),
    new Job(4, 7),
    new Job(5, 1),
  ];

  const workers = [
    new Worker(0),
    new Worker(1),
  ];

  return {
    jobs,
    workers,
    priorities: [5, 9, 3, 7, 1],
    statuses: [0, 0, 0, 0, 0], // 0=Pending, 1=Running, 2=Complete, 3=Failed
    step: 0,
    logs: ['Demo initialized with 5 jobs and 2 workers'],
  };
}

function getStatusString(statusCode) {
  const statusMap = { 0: 'Pending', 1: 'Running', 2: 'Complete', 3: 'Failed' };
  return statusMap[statusCode] || 'Unknown';
}

function getCurrentState() {
  if (!demoState) {
    demoState = initializeDemo();
  }

  // Build response with current state
  const jobsInfo = demoState.jobs.map((job, idx) => ({
    id: job.get_id(),
    priority: job.get_priority(),
    status: getStatusString(demoState.statuses[idx]),
    statusCode: job.status_code(),
    isPending: job.is_pending(),
  }));

  const workersInfo = demoState.workers.map(worker => ({
    id: worker.get_id(),
    isIdle: worker.is_idle(),
    activeJobId: worker.active_job_id,
    completedCount: worker.get_completed_count(),
  }));

  const stats = {
    pending: count_by_status(demoState.statuses, 0, 5),
    running: count_by_status(demoState.statuses, 1, 5),
    completed: count_by_status(demoState.statuses, 2, 5),
    failed: count_by_status(demoState.statuses, 3, 5),
  };

  return {
    jobs: jobsInfo,
    workers: workersInfo,
    stats,
    step: demoState.step,
    logs: demoState.logs,
    isDone: demoState.step >= 8,
  };
}

function advanceStep() {
  if (!demoState) {
    demoState = initializeDemo();
  }

  const step = demoState.step;
  const logs = demoState.logs;

  try {
    switch (step) {
      case 0: {
        // Step 1: Find highest priority job
        const best = find_highest_priority(demoState.priorities, demoState.statuses, 5);
        logs.push(`Step 1: Found highest priority job at index ${best} (Job ${best + 1}, priority ${demoState.priorities[best]})`);
        demoState.step++;
        break;
      }

      case 1: {
        // Step 2: Assign highest priority job (Job 2) to Worker 0
        const job = demoState.jobs[1]; // Job 2 (index 1)
        const worker = demoState.workers[0]; // Worker 0
        job.assign(worker.get_id());
        worker.start_job(job.get_id());
        demoState.statuses[1] = 1; // Running
        logs.push(`Step 2: Worker 0 assigned Job 2 (priority 9)`);
        demoState.step++;
        break;
      }

      case 2: {
        // Step 3: Worker 0 completes Job 2
        const job = demoState.jobs[1];
        const worker = demoState.workers[0];
        job.complete();
        worker.finish_job();
        demoState.statuses[1] = 2; // Complete
        logs.push(`Step 3: Worker 0 completed Job 2 (total completed: ${worker.get_completed_count()})`);
        demoState.step++;
        break;
      }

      case 3: {
        // Step 4: Find next highest priority job and assign to Worker 1
        const next = find_highest_priority(demoState.priorities, demoState.statuses, 5);
        const job = demoState.jobs[next]; // Job 4 (index 3)
        const worker = demoState.workers[1]; // Worker 1
        job.assign(worker.get_id());
        worker.start_job(job.get_id());
        demoState.statuses[next] = 1; // Running
        logs.push(`Step 4: Worker 1 assigned Job 4 (priority 7)`);
        demoState.step++;
        break;
      }

      case 4: {
        // Step 5: Worker 1's Job 4 fails
        const job = demoState.jobs[3]; // Job 4
        const worker = demoState.workers[1];
        job.fail(42);
        worker.finish_job();
        demoState.statuses[3] = 3; // Failed
        logs.push(`Step 5: Worker 1: Job 4 failed with error code 42`);
        demoState.step++;
        break;
      }

      case 5: {
        // Step 6: Assign remaining jobs - Job 1
        const next = find_highest_priority(demoState.priorities, demoState.statuses, 5);
        const job = demoState.jobs[next]; // Job 1 (index 0)
        const worker = demoState.workers[0]; // Worker 0 is idle
        job.assign(worker.get_id());
        worker.start_job(job.get_id());
        demoState.statuses[next] = 1;
        logs.push(`Step 6: Worker 0 assigned Job 1 (priority 5)`);
        demoState.step++;
        break;
      }

      case 6: {
        // Step 7: Complete Job 1 and assign Job 3
        const job1 = demoState.jobs[0];
        const worker0 = demoState.workers[0];
        job1.complete();
        worker0.finish_job();
        demoState.statuses[0] = 2;

        const next = find_highest_priority(demoState.priorities, demoState.statuses, 5);
        const job3 = demoState.jobs[next]; // Job 3 (index 2)
        job3.assign(worker0.get_id());
        worker0.start_job(job3.get_id());
        demoState.statuses[next] = 1;
        logs.push(`Step 7: Worker 0 completed Job 1, assigned Job 3 (priority 3)`);
        demoState.step++;
        break;
      }

      case 7: {
        // Step 8: Complete Job 3 and assign Job 5
        const job3 = demoState.jobs[2];
        const worker0 = demoState.workers[0];
        job3.complete();
        worker0.finish_job();
        demoState.statuses[2] = 2;

        const next = find_highest_priority(demoState.priorities, demoState.statuses, 5);
        const job5 = demoState.jobs[next]; // Job 5 (index 4)
        job5.assign(worker0.get_id());
        worker0.start_job(job5.get_id());
        demoState.statuses[next] = 1;
        logs.push(`Step 8: Worker 0 completed Job 3, assigned Job 5 (priority 1)`);
        demoState.step++;
        break;
      }

      case 8: {
        // Step 9: Complete Job 5 - all done
        const job5 = demoState.jobs[4];
        const worker0 = demoState.workers[0];
        job5.complete();
        worker0.finish_job();
        demoState.statuses[4] = 2;
        logs.push(`Step 9: Worker 0 completed Job 5. Demo complete!`);
        logs.push(`Final stats - Worker 0: ${worker0.get_completed_count()} jobs, Worker 1: ${demoState.workers[1].get_completed_count()} job`);
        demoState.step++;
        break;
      }

      default:
        logs.push('Demo complete. Reset to run again.');
        break;
    }
  } catch (error) {
    logs.push(`ERROR: ${error.message}`);
  }

  return getCurrentState();
}

function tryBreakContract() {
  // Attempt to create a job with invalid priority (breaks contract)
  const logs = [];
  try {
    logs.push('Attempting to create Job with priority 0 (violates contract: priority >= 1)...');
    const badJob = new Job(99, 0); // This should throw
    logs.push('ERROR: Contract was not enforced!');
  } catch (error) {
    logs.push(`SUCCESS: Contract enforced! Error: ${error.message}`);
  }

  try {
    logs.push('Attempting to create Job with priority 11 (violates contract: priority <= 10)...');
    const badJob = new Job(100, 11); // This should throw
    logs.push('ERROR: Contract was not enforced!');
  } catch (error) {
    logs.push(`SUCCESS: Contract enforced! Error: ${error.message}`);
  }

  // Try to assign job to a busy worker
  try {
    logs.push('Attempting to call finish_job on an idle worker (violates precondition)...');
    const testWorker = new Worker(99);
    testWorker.finish_job(); // This should throw
    logs.push('ERROR: Precondition was not enforced!');
  } catch (error) {
    logs.push(`SUCCESS: Precondition enforced! Error: ${error.message}`);
  }

  return { success: true, logs };
}

// HTTP Server
const server = http.createServer((req, res) => {
  // CORS headers for local development
  res.setHeader('Access-Control-Allow-Origin', '*');
  res.setHeader('Access-Control-Allow-Methods', 'GET, POST, OPTIONS');
  res.setHeader('Access-Control-Allow-Headers', 'Content-Type');

  if (req.method === 'OPTIONS') {
    res.writeHead(200);
    res.end();
    return;
  }

  // Route handling
  if (req.method === 'GET' && req.url === '/') {
    // Serve the dashboard HTML
    const html = fs.readFileSync(path.join(__dirname, 'index.html'), 'utf8');
    res.writeHead(200, { 'Content-Type': 'text/html' });
    res.end(html);
  } else if (req.method === 'GET' && req.url === '/api/state') {
    // Get current state
    const state = getCurrentState();
    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify(state));
  } else if (req.method === 'POST' && req.url === '/api/step') {
    // Advance one step
    const state = advanceStep();
    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify(state));
  } else if (req.method === 'POST' && req.url === '/api/reset') {
    // Reset the demo
    demoState = initializeDemo();
    const state = getCurrentState();
    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify(state));
  } else if (req.method === 'POST' && req.url === '/api/break') {
    // Try to break a contract
    const result = tryBreakContract();
    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify(result));
  } else {
    res.writeHead(404, { 'Content-Type': 'text/plain' });
    res.end('Not Found');
  }
});

const PORT = process.env.PORT || 3000;

server.listen(PORT, () => {
  console.log(`Intent Task Queue Server running at http://localhost:${PORT}/`);
  console.log(`Using compiler-generated code from: task_queue.generated.js`);
  console.log('');
  console.log('API Endpoints:');
  console.log(`  GET  /              - Dashboard UI`);
  console.log(`  GET  /api/state     - Get current state`);
  console.log(`  POST /api/step      - Advance one step`);
  console.log(`  POST /api/reset     - Reset demo`);
  console.log(`  POST /api/break     - Try breaking contracts`);
});
