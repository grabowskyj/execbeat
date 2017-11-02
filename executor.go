package beat

import (
	"bytes"
	"github.com/christiangalsterer/execbeat/config"
	"github.com/elastic/beats/libbeat/logp"
	"github.com/robfig/cron"
	"os/exec"
	"io"
	"strings"
	"syscall"
	"time"
)

type Executor struct {
	execbeat     *Execbeat
	config       config.ExecConfig
	schedule     string
	documentType string
	timeout      string
}

func NewExecutor(execbeat *Execbeat, config config.ExecConfig) *Executor {
	executor := &Executor{
		execbeat: execbeat,
		config:   config,
	}

	return executor
}

func (e *Executor) Run() {

	// setup default config
	e.documentType = config.DefaultDocumentType
	e.schedule = config.DefaultSchedule

	// setup document type
	if e.config.DocumentType != "" {
		e.documentType = e.config.DocumentType
	}

	// setup cron schedule
	if e.config.Schedule != "" {
		logp.Debug("Execbeat", "Use schedule: [%w]", e.config.Schedule)
		e.schedule = e.config.Schedule
	}
        
        // setup command execution time
        if e.config.Timeout != "" {
                logp.Debug("Execbeat", "Command timeout: [%w]", e.config.Timeout)
                e.timeout = e.config.Timeout
        }

        cron := cron.New()
        cron.AddFunc(e.schedule, func() { e.runOneTime() })
        cron.Start()

}

func getUsDuration(beginning time.Time) int64 {
        durationSince := time.Since(beginning)
        nsDuration := durationSince.Nanoseconds()
        usDuration := nsDuration / 1000
    
        return usDuration
}

func (e *Executor) runOneTime() error {
	var cmd *exec.Cmd
	var cmdArgs []string
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	var waitStatus syscall.WaitStatus
	var exitCode int = 0
	var duration int64 = 0
	var timeoutStr string
	var timeoutValue time.Duration
	var errTimeout error

	cmdName := strings.TrimSpace(e.config.Command)

	args := strings.TrimSpace(e.config.Args)
	if len(args) > 0 {
		cmdArgs = strings.Split(args, " ")
	}

        // parse timeout string to time.Duration
        if e.timeout != "" {
                timeoutStr = e.timeout
                timeoutValue, errTimeout = time.ParseDuration(timeoutStr)

                if errTimeout != nil {
                             logp.Err("Execbeat", "Not valid timeout duration: %v", timeoutStr)   
                }
        }
        
	// execute command
	now := time.Now()

	if len(cmdArgs) > 0 {
		logp.Debug("Execbeat", "Executing command: [%v] with args [%w]", cmdName, cmdArgs)
		cmd = exec.Command(cmdName, cmdArgs...)
	} else {
		logp.Debug("Execbeat", "Executing command: [%v]", cmdName)
		cmd = exec.Command(cmdName)
	}
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

        cmd.Start()
        
        doneCh := make(chan error)       
        go func() { doneCh <-cmd.Wait() }()

	logp.Info("Execbeat", "Executing command: [%v]", stdout.String())
        
        if e.timeout != "" {
                select {
                case <-time.After(timeoutValue):
                        duration = getUsDuration(now)
			cmd.Process.Kill()
                                                
                        logp.Err("Command execution has timed out after %v", timeoutStr)
                        io.WriteString(cmd.Stderr, "Command execution has timed out")
                        exitCode = 124
                case errDCh := <-doneCh:
                        duration = getUsDuration(now)
                        
                        if errDCh != nil {
                                logp.Err("An error occured while executing command: %v", errDCh)
                		exitCode = 127
                		if exitError, ok := errDCh.(*exec.ExitError); ok {
	                		waitStatus = exitError.Sys().(syscall.WaitStatus)
	                		exitCode = waitStatus.ExitStatus()
                		}       
                        }                     
                }
        } else {
                select {
                        case errDCh := <-doneCh:
                        duration = getUsDuration(now)
                        
                        if errDCh != nil {
                                logp.Err("An error occured while executing command: %v", errDCh)
                		exitCode = 127
                		if exitError, ok := errDCh.(*exec.ExitError); ok {
	                		waitStatus = exitError.Sys().(syscall.WaitStatus)
	                		exitCode = waitStatus.ExitStatus()
                		}       
                        }
                }        
        }

	commandEvent := Exec{
		Command:  cmdName,
		StdOut:   stdout.String(),
		StdErr:   stderr.String(),
		ExitCode: exitCode,
                Duration: duration,
	}

	event := ExecEvent{
		ReadTime:     now,
		DocumentType: e.documentType,
		Fields:       e.config.Fields,
		Exec:         commandEvent,
	}

	e.execbeat.client.PublishEvent(event.ToMapStr())

	return nil
}

func (e *Executor) Stop() {
}
