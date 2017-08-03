package backend

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/digitalrebar/store"
	"github.com/digitalrebar/provision/backend/index"
	"github.com/pborman/uuid"
)

// Job represents a task that is running (or has run) on a machine.
// The job create workflow I envision works like this:
//
// * POST to api/v3/jobs with a body containing {"Machine":
//   "a-machine-uuid"} If there is no current job, or the current job
//   is "failed", a new job is created for the Task indexed by
//   CurrentTask. If the current job is "finished", the machine
//   CurrentTask is incremented.  If that causes CurrentTask to go
//   past the end of the Tasks list for the machine, no job is created
//   and the API returns a 204. If the current job is in the imcomplete
//   state, that job is returned with a 202.  Otherwise a new job is
//   created and is returned with a 201. If there is a current job that is neither
//   "completed", "failed", nor "finished", the POST fails.  The new job will be
//   created with its Previous value set to the machine's CurrentJob,
//   and the machine's CurrentJob is updated with the UUID of the new
//   job.
//
// * When a new Job is created, it makes a RenderData for the
//   templates contained in the Task the job was created against.  The
//   client will be able to retrieve the rendered templates via GET
//   from api/v3/jobs/:job-id/templates.
//
// * The client will place or execute the templates based on whether
//   there is a Path associated with the expanded Template in the
//   order that the jobs/:id/templates API endpoint returns them in.
//   As it does so, it will log its progress via POST to jobs/:id/log.
//
// * If any job operation fails, the client will update the job status to "failed".
//
// * If all job operations succeed, the client will update the job status to "finished"
//
// * On provisioner startup, all machine CurrentJobs are set to "failed" if they are not "finished"
//
// swagger:model
type Job struct {
	validate

	// The UUID of the job.  The primary key.
	// required: true
	// swagger:strfmt uuid
	Uuid uuid.UUID
	// The UUID of the previous job to run on this machine.
	// swagger:strfmt uuid
	Previous uuid.UUID
	// The machine the job was created for.  This field must be the UUID of the machine.
	// required: true
	// swagger:strfmt uuid
	Machine uuid.UUID
	// The task the job was created for.  This will be the name of the task.
	// read only: true
	Task string
	// The boot environment that the task was created in.
	// read only: true
	BootEnv string
	// The state the job is in.  Must be one of "created", "running", "failed", "finished", "incomplete"
	// required: true
	State string
	// The time the job entered running.
	StartTime time.Time
	// The time the job entered failed or finished.
	EndTime time.Time
	// required: true
	Archived bool
	// DRP Filesystem path to the log for this job
	// read only: true
	LogPath string

	p        *DataTracker
	oldState string
}

// Job Action is something that job runner will need to do.
// If path is specified, then the runner will place the contents into that location.
// If path is not specified, then the runner will attempt to bash exec the contents.
// swagger:model
type JobAction struct {
	// required: true
	Name string
	// required: true
	Path string
	// required: true
	Content string
}

func AsJob(o store.KeySaver) *Job {
	return o.(*Job)
}

func AsJobs(o []store.KeySaver) []*Job {
	res := make([]*Job, len(o))
	for i := range o {
		res[i] = AsJob(o[i])
	}
	return res
}

func (j *Job) Backend() store.SimpleStore {
	return j.p.getBackend(j)
}

func (j *Job) Prefix() string {
	return "jobs"
}

func (j *Job) Key() string {
	return j.Uuid.String()
}

func (j *Job) AuthKey() string {
	return j.Machine.String()
}

func (j *Job) New() store.KeySaver {
	res := &Job{p: j.p}
	return store.KeySaver(res)
}

func (d *DataTracker) NewJob() *Job {
	return &Job{p: d}
}

func (j *Job) setDT(dp *DataTracker) {
	j.p = dp
}

func (j *Job) UUID() string {
	return j.Uuid.String()
}

func (j *Job) Indexes() map[string]index.Maker {
	fix := AsJob
	return map[string]index.Maker{
		"Key": index.MakeKey(),
		"Uuid": index.Make(
			true,
			"UUID string",
			func(i, j store.KeySaver) bool { return fix(i).Uuid.String() < fix(j).Uuid.String() },
			func(ref store.KeySaver) (gte, gt index.Test) {
				refUuid := fix(ref).Uuid.String()
				return func(s store.KeySaver) bool {
						return fix(s).Uuid.String() >= refUuid
					},
					func(s store.KeySaver) bool {
						return fix(s).Uuid.String() > refUuid
					}
			},
			func(s string) (store.KeySaver, error) {
				id := uuid.Parse(s)
				if id == nil {
					return nil, fmt.Errorf("Invalid UUID: %s", s)
				}
				return &Job{Uuid: id}, nil
			}),
		"BootEnv": index.Make(
			false,
			"string",
			func(i, j store.KeySaver) bool { return fix(i).BootEnv < fix(j).BootEnv },
			func(ref store.KeySaver) (gte, gt index.Test) {
				refBootEnv := fix(ref).BootEnv
				return func(s store.KeySaver) bool {
						return fix(s).BootEnv >= refBootEnv
					},
					func(s store.KeySaver) bool {
						return fix(s).BootEnv > refBootEnv
					}
			},
			func(s string) (store.KeySaver, error) {
				return &Job{BootEnv: s}, nil
			}),
		"Task": index.Make(
			false,
			"string",
			func(i, j store.KeySaver) bool { return fix(i).Task < fix(j).Task },
			func(ref store.KeySaver) (gte, gt index.Test) {
				refTask := fix(ref).Task
				return func(s store.KeySaver) bool {
						return fix(s).Task >= refTask
					},
					func(s store.KeySaver) bool {
						return fix(s).Task > refTask
					}
			},
			func(s string) (store.KeySaver, error) {
				return &Job{Task: s}, nil
			}),
		"State": index.Make(
			false,
			"string",
			func(i, j store.KeySaver) bool { return fix(i).State < fix(j).State },
			func(ref store.KeySaver) (gte, gt index.Test) {
				refState := fix(ref).State
				return func(s store.KeySaver) bool {
						return fix(s).State >= refState
					},
					func(s store.KeySaver) bool {
						return fix(s).State > refState
					}
			},
			func(s string) (store.KeySaver, error) {
				return &Job{State: s}, nil
			}),
		"Machine": index.Make(
			true,
			"UUID string",
			func(i, j store.KeySaver) bool { return fix(i).Machine.String() < fix(j).Machine.String() },
			func(ref store.KeySaver) (gte, gt index.Test) {
				refMachine := fix(ref).Machine.String()
				return func(s store.KeySaver) bool {
						return fix(s).Machine.String() >= refMachine
					},
					func(s store.KeySaver) bool {
						return fix(s).Machine.String() > refMachine
					}
			},
			func(s string) (store.KeySaver, error) {
				id := uuid.Parse(s)
				if id == nil {
					return nil, fmt.Errorf("Invalid UUID: %s", s)
				}
				return &Job{Machine: id}, nil
			}),
		"Archived": index.Make(
			false,
			"boolean",
			func(i, j store.KeySaver) bool {
				return (!fix(i).Archived) && fix(j).Archived
			},
			func(ref store.KeySaver) (gte, gt index.Test) {
				avail := fix(ref).Archived
				return func(s store.KeySaver) bool {
						v := fix(s).Archived
						return v || (v == avail)
					},
					func(s store.KeySaver) bool {
						return fix(s).Archived && !avail
					}
			},
			func(s string) (store.KeySaver, error) {
				res := &Job{}
				switch s {
				case "true":
					res.Archived = true
				case "false":
					res.Archived = false
				default:
					return nil, errors.New("Archived must be true or false")
				}
				return res, nil
			}),
		"StartTime": index.Make(
			false,
			"dateTime",
			func(i, j store.KeySaver) bool {
				return fix(i).StartTime.Before(fix(j).StartTime)
			},
			func(ref store.KeySaver) (gte, gt index.Test) {
				refTime := fix(ref).StartTime
				return func(s store.KeySaver) bool {
						cmpTime := fix(s).StartTime
						return refTime.Equal(cmpTime) || cmpTime.After(refTime)
					},
					func(s store.KeySaver) bool {
						return fix(s).StartTime.After(refTime)
					}
			},
			func(s string) (store.KeySaver, error) {
				parsedTime, err := time.Parse(time.RFC3339, s)
				if err != nil {
					return nil, err
				}
				return &Job{StartTime: parsedTime}, nil
			}),
		"EndTime": index.Make(
			false,
			"dateTime",
			func(i, j store.KeySaver) bool {
				return fix(i).EndTime.Before(fix(j).EndTime)
			},
			func(ref store.KeySaver) (gte, gt index.Test) {
				refTime := fix(ref).EndTime
				return func(s store.KeySaver) bool {
						cmpTime := fix(s).EndTime
						return refTime.Equal(cmpTime) || cmpTime.After(refTime)
					},
					func(s store.KeySaver) bool {
						return fix(s).EndTime.After(refTime)
					}
			},
			func(s string) (store.KeySaver, error) {
				parsedTime, err := time.Parse(time.RFC3339, s)
				if err != nil {
					return nil, err
				}
				return &Job{EndTime: parsedTime}, nil
			}),
	}
}

var JobValidStates []string = []string{
	"created",
	"running",
	"failed",
	"finished",
	"incomplete",
}

func (j *Job) OnChange(oldThing store.KeySaver) error {
	j.oldState = AsJob(oldThing).State
	return nil
}

func (j *Job) BeforeSave() error {
	e := &Error{Code: 422, Type: ValidationError, o: j}
	if j.Uuid == nil {
		e.Errorf("Job %#v was not assigned a uuid!", j)
	}
	if j.Previous == nil {
		e.Errorf("Job %s does not have a Previous job", j.UUID())
	}

	objs := j.stores
	tasks := objs("tasks")
	bootenvs := objs("bootenvs")
	machines := objs("machines")

	var m *Machine
	if om := machines.Find(j.Machine.String()); om == nil {
		e.Errorf("Machine %s does not exist", j.Machine.String())
	} else {
		m = AsMachine(om)
	}

	if tasks.Find(j.Task) == nil {
		e.Errorf("Task %s does not exist", j.Task)
	}

	var env *BootEnv
	if nbFound := bootenvs.Find(j.BootEnv); nbFound == nil {
		e.Errorf("Bootenv %s does not exist", j.BootEnv)
	} else {
		env = AsBootEnv(nbFound)
	}
	if env != nil && !env.Available {
		e.Errorf("Jobs %s wants BootEnv %s, which is not available", j.UUID(), j.BootEnv)
	}

	found := false
	for _, s := range JobValidStates {
		if s == j.State {
			found = true
			break
		}
	}
	if !found {
		e.Errorf("Jobs %s wants State %s, which is not valid", j.UUID(), j.State)
	}

	if j.LogPath == "" {
		j.LogPath = filepath.Join(j.p.LogRoot, j.Uuid.String())
		buf := &bytes.Buffer{}
		fmt.Fprintf(buf, "Log for Job: %s\n", j.Uuid.String())
		ee := j.Log(buf)
		e.Merge(ee)
	}

	if e.OrNil() == nil {
		if j.oldState != j.State {
			if j.State == "running" {
				j.StartTime = time.Now()
			}
			if j.State == "failed" || j.State == "finished" {
				j.EndTime = time.Now()
			}
			// We are going to failed.  Mark machine as not Runnable
			if j.State == "failed" {
				m.Runnable = false
				_, e2 := j.p.Save(objs, m, nil)
				e.Merge(e2)
			}
		}
	}

	return e.OrNil()
}

func (j *Job) BeforeDelete() error {
	e := &Error{Code: 422, Type: ValidationError, o: j}

	if j.State != "finished" && j.State != "failed" {
		e.Errorf("Jobs %s is not in a deletable state: %s", j.UUID(), j.State)
	}

	return e.OrNil()
}

func (j *Job) RenderActions() ([]*JobAction, error) {
	renderers, addr, e := func() (renderers, net.IP, error) {
		d, unlocker := j.p.LockEnts(j.Locks("actions")...)
		defer unlocker()
		machines := d("machines")
		tasks := d("tasks")

		// This should not happen, but we treat task in the job as soft.
		var to store.KeySaver
		if to = tasks.Find(j.Task); to == nil {
			err := &Error{Code: http.StatusUnprocessableEntity, Type: ValidationError,
				Messages: []string{fmt.Sprintf("Task %s does not exist", j.Task)}}
			return nil, nil, err
		}
		t := AsTask(to)

		// This should not happen, but we treat machine in the job as soft.
		var mo store.KeySaver
		if mo = machines.Find(j.Machine.String()); mo == nil {
			err := &Error{Code: http.StatusUnprocessableEntity, Type: ValidationError,
				Messages: []string{fmt.Sprintf("Machine %s does not exist", j.Machine.String())}}
			return nil, nil, err
		}
		m := AsMachine(mo)

		err := &Error{}
		renderers := t.Render(d, m, err)
		if err.OrNil() != nil {
			return nil, nil, err
		}

		return renderers, m.Address, nil
	}()
	if e != nil {
		return nil, e
	}

	err := &Error{}
	actions := []*JobAction{}
	for _, r := range renderers {
		rr, err1 := r.write(addr)
		if err1 != nil {
			err.Merge(err1)
		} else {
			b, err2 := ioutil.ReadAll(rr)
			if err2 != nil {
				err.Merge(err2)
			} else {
				na := &JobAction{Name: r.name, Path: r.path, Content: string(b)}
				actions = append(actions, na)
			}
		}
	}

	return actions, err.OrNil()
}

func (j *Job) Log(src io.Reader) error {
	f, err := os.OpenFile(j.LogPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		fmt.Printf("Umm err: %v\n", err)
		return err
	}
	_, err = io.Copy(f, src)
	if err != nil {
		fmt.Printf("Umm write err: %v\n", err)
		return err
	}
	return nil
}

var jobLockMap = map[string][]string{
	"get":     []string{"jobs"},
	"create":  []string{"jobs", "machines", "tasks", "bootenvs", "profiles"},
	"update":  []string{"jobs", "machines", "tasks", "bootenvs", "profiles"},
	"patch":   []string{"jobs", "machines", "tasks", "bootenvs", "profiles"},
	"delete":  []string{"jobs"},
	"actions": []string{"jobs", "machines", "tasks", "profiles"},
}

func (j *Job) Locks(action string) []string {
	return jobLockMap[action]
}
