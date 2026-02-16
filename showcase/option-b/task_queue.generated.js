// Generated JavaScript code from Intent

/**
 * Enum: JobStatus
 */
const JobStatus = {
  Pending: () => ({ _tag: "Pending" }),
  Running: (worker_id) => ({ _tag: "Running", worker_id }),
  Complete: () => ({ _tag: "Complete" }),
  Failed: (error_code) => ({ _tag: "Failed", error_code }),
};

/**
 * Entity: Job
 */
class Job {
  /**
   * Constructor
   * @param {number} id
   * @param {number} priority
   */
  constructor(id, priority) {
    if (!((priority >= 1))) throw new Error("Precondition failed: priority >= 1");
    if (!((priority <= 10))) throw new Error("Precondition failed: priority <= 10");
    this.id = 0;
    this.priority = 0;
    this.status = null;
    this.id = id;
    this.priority = priority;
    this.status = JobStatus.Pending();
    if (!((this.id === id))) throw new Error("Postcondition failed: self . id == id");
    if (!((this.priority === priority))) throw new Error("Postcondition failed: self . priority == priority");
    this.__checkInvariants();
  }

  /**
   * Check invariants
   */
  __checkInvariants() {
    if (!((this.priority >= 1))) throw new Error("Invariant failed: self . priority >= 1");
    if (!((this.priority <= 10))) throw new Error("Invariant failed: self . priority <= 10");
  }

  /**
   * Method: get_id
   * @returns {number}
   */
  get_id() {
    let __result;
    {
      return this.id;
    }
    this.__checkInvariants();
    return __result;
  }

  /**
   * Method: get_priority
   * @returns {number}
   */
  get_priority() {
    let __result;
    {
      return this.priority;
    }
    this.__checkInvariants();
    return __result;
  }

  /**
   * Method: is_pending
   * @returns {boolean}
   */
  is_pending() {
    let __result;
    {
      let code = (() => {
  const __scrutinee = this.status;
  if (__scrutinee._tag === "Pending") {
    return 1;
  }
  else if (__scrutinee._tag === "Running") {
    const w = __scrutinee.worker_id;
    return 0;
  }
  else if (__scrutinee._tag === "Complete") {
    return 0;
  }
  else if (__scrutinee._tag === "Failed") {
    const r = __scrutinee.error_code;
    return 0;
  }
})();
      return (code === 1);
    }
    this.__checkInvariants();
    return __result;
  }

  /**
   * Method: assign
   * @param {number} worker_id
   * @returns {void}
   */
  assign(worker_id) {
    if (!((worker_id >= 0))) throw new Error("Precondition failed: worker_id >= 0");
    this.status = JobStatus.Running(worker_id);
    this.__checkInvariants();
  }

  /**
   * Method: complete
   * @returns {void}
   */
  complete() {
    this.status = JobStatus.Complete();
    this.__checkInvariants();
  }

  /**
   * Method: fail
   * @param {number} error_code
   * @returns {void}
   */
  fail(error_code) {
    this.status = JobStatus.Failed(error_code);
    this.__checkInvariants();
  }

  /**
   * Method: status_code
   * @returns {number}
   */
  status_code() {
    let __result;
    {
      return (() => {
  const __scrutinee = this.status;
  if (__scrutinee._tag === "Pending") {
    return 0;
  }
  else if (__scrutinee._tag === "Running") {
    const w = __scrutinee.worker_id;
    return 1;
  }
  else if (__scrutinee._tag === "Complete") {
    return 2;
  }
  else if (__scrutinee._tag === "Failed") {
    const r = __scrutinee.error_code;
    return 3;
  }
})();
    }
    this.__checkInvariants();
    return __result;
  }

}

/**
 * Entity: Worker
 */
class Worker {
  /**
   * Constructor
   * @param {number} id
   */
  constructor(id) {
    this.id = 0;
    this.active_job_id = 0;
    this.jobs_completed = 0;
    this.is_busy = false;
    this.id = id;
    this.active_job_id = -1;
    this.jobs_completed = 0;
    this.is_busy = false;
    if (!((this.id === id))) throw new Error("Postcondition failed: self . id == id");
    if (!((this.jobs_completed === 0))) throw new Error("Postcondition failed: self . jobs_completed == 0");
    this.__checkInvariants();
  }

  /**
   * Check invariants
   */
  __checkInvariants() {
    if (!((this.jobs_completed >= 0))) throw new Error("Invariant failed: self . jobs_completed >= 0");
  }

  /**
   * Method: get_id
   * @returns {number}
   */
  get_id() {
    let __result;
    {
      return this.id;
    }
    this.__checkInvariants();
    return __result;
  }

  /**
   * Method: is_idle
   * @returns {boolean}
   */
  is_idle() {
    let __result;
    {
      return (this.is_busy === false);
    }
    this.__checkInvariants();
    return __result;
  }

  /**
   * Method: start_job
   * @param {number} job_id
   * @returns {void}
   */
  start_job(job_id) {
    if (!((this.is_busy === false))) throw new Error("Precondition failed: self . is_busy == false");
    this.active_job_id = job_id;
    this.is_busy = true;
    this.__checkInvariants();
  }

  /**
   * Method: finish_job
   * @returns {void}
   */
  finish_job() {
    const __old_self_jobs_completed = this.jobs_completed;
    if (!((this.is_busy === true))) throw new Error("Precondition failed: self . is_busy == true");
    this.active_job_id = -1;
    this.is_busy = false;
    this.jobs_completed = (this.jobs_completed + 1);
    if (!((this.jobs_completed === (__old_self_jobs_completed + 1)))) throw new Error("Postcondition failed: self . jobs_completed == old ( self . jobs_completed ) + 1");
    this.__checkInvariants();
  }

  /**
   * Method: get_completed_count
   * @returns {number}
   */
  get_completed_count() {
    let __result;
    {
      return this.jobs_completed;
    }
    this.__checkInvariants();
    return __result;
  }

}

/**
 * @param {Array<number>} priorities
 * @param {Array<number>} statuses
 * @param {number} count
 * @returns {number}
 */
function find_highest_priority(priorities, statuses, count) {
  if (!((count >= 0))) throw new Error("Precondition failed: count >= 0");
  if (!(((priorities.length) >= count))) throw new Error("Precondition failed: len ( priorities ) >= count");
  if (!(((statuses.length) >= count))) throw new Error("Precondition failed: len ( statuses ) >= count");
  let best_idx = -1;
  let best_pri = 0;
  let i = 0;
  while ((i < count)) {
    if ((statuses[i] === 0)) {
      if ((priorities[i] > best_pri)) {
        best_pri = priorities[i];
        best_idx = i;
      }
    }
    i = (i + 1);
  }
  return best_idx;
}

/**
 * @param {Array<number>} statuses
 * @param {number} target_status
 * @param {number} count
 * @returns {number}
 */
function count_by_status(statuses, target_status, count) {
  if (!((count >= 0))) throw new Error("Precondition failed: count >= 0");
  if (!(((statuses.length) >= count))) throw new Error("Precondition failed: len ( statuses ) >= count");
  let __result;
  {
    let total = 0;
    let i = 0;
    while ((i < count)) {
      if ((statuses[i] === target_status)) {
        total = (total + 1);
      }
      i = (i + 1);
    }
    return total;
  }
  if (!((__result >= 0))) throw new Error("Postcondition failed: result >= 0");
  return __result;
}

/**
 * Entry function
 * @returns {number}
 */
function __intent_main() {
  console.log("=== Intent Task Queue Demo ===");
  let job1 = new Job(1, 5);
  let job2 = new Job(2, 9);
  let job3 = new Job(3, 3);
  let job4 = new Job(4, 7);
  let job5 = new Job(5, 1);
  let priorities = [5, 9, 3, 7, 1];
  let statuses = [0, 0, 0, 0, 0];
  console.log("Jobs created: 5");
  console.log("Queue capacity: 10");
  let worker1 = new Worker(0);
  let worker2 = new Worker(1);
  let best = find_highest_priority(priorities, statuses, 5);
  console.log("Highest priority job index:");
  console.log(best);
  job2.assign(worker1.get_id());
  worker1.start_job(job2.get_id());
  statuses[1] = 1;
  console.log("Worker 0 assigned job 2 (priority 9)");
  job2.complete();
  worker1.finish_job();
  statuses[1] = 2;
  console.log("Worker 0 completed job 2");
  console.log("Worker 0 total completed:");
  console.log(worker1.get_completed_count());
  let next = find_highest_priority(priorities, statuses, 5);
  console.log("Next highest priority job index:");
  console.log(next);
  job4.assign(worker2.get_id());
  worker2.start_job(job4.get_id());
  statuses[3] = 1;
  console.log("Worker 1 assigned job 4 (priority 7)");
  job4.fail(42);
  worker2.finish_job();
  statuses[3] = 3;
  console.log("Worker 1: job 4 failed (error 42)");
  console.log("Worker 1 total completed:");
  console.log(worker2.get_completed_count());
  let pending = count_by_status(statuses, 0, 5);
  let running = count_by_status(statuses, 1, 5);
  let completed = count_by_status(statuses, 2, 5);
  let failed = count_by_status(statuses, 3, 5);
  console.log("--- Status Summary ---");
  console.log("Pending:");
  console.log(pending);
  console.log("Running:");
  console.log(running);
  console.log("Completed:");
  console.log(completed);
  console.log("Failed:");
  console.log(failed);
  console.log("--- Job Details ---");
  console.log("Job 1 status:");
  console.log(job1.status_code());
  console.log("Job 2 status:");
  console.log(job2.status_code());
  console.log("Job 3 status:");
  console.log(job3.status_code());
  console.log("Job 4 status:");
  console.log(job4.status_code());
  console.log("Job 5 status:");
  console.log(job5.status_code());
  console.log("=== Demo Complete ===");
  return 0;
}

// Entry point invocation
const __exitCode = __intent_main();
process.exit(__exitCode);
