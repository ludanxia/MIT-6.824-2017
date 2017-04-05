package raft

//
// this is an outline of the API that raft must expose to
// the service (or tester). see comments below for
// each of these functions for more details.
//
// rf = Make(...)
//   create a new Raft server.
// rf.Start(command interface{}) (index, term, isleader)
//   start agreement on a new log entry
// rf.GetState() (term, isLeader)
//   ask a Raft for its current term, and whether it thinks it is leader
// ApplyMsg
//   each time a new entry is committed to the log, each Raft peer
//   should send an ApplyMsg to the service (or tester)
//   in the same server.
//
import (
	"fmt"
	"labrpc"
	"math/rand"
	"sync"
	"time"
)

// import "bytes"
// import "encoding/gob"

//
// as each Raft peer becomes aware that successive log entries are
// committed, the peer should send an ApplyMsg to the service (or
// tester) on the same server, via the applyCh passed to Make().
//
type ApplyMsg struct {
	Index       int
	Command     interface{}
	UseSnapshot bool   // ignore for lab2; only used in lab3
	Snapshot    []byte // ignore for lab2; only used in lab3
}

//
// The log entery struct
//
type Entry struct {
	Term    int
	Command int
}

//
// A Go object implementing a single Raft peer.
//
type Raft struct {
	sync.Mutex                     // Lock to protect shared access to this peer's state
	peers      []*labrpc.ClientEnd // RPC end points of all peers
	persister  *Persister          // Object to hold this peer's persisted state
	me         int                 // this peer's index into peers[]

	// Your data here (2A, 2B, 2C).
	// Look at the paper's Figure 2 for a description of what
	// state a Raft server must maintain.
	//LAB 2A
	status      int           //The status of raft: leader, candidate, or follower
	currentTerm int           //Latest term server has seen
	votedFor    int           //CandidateID that receive vote in current term
	log         map[int]Entry //log
	timeoutT    *time.Timer   //Timer to kick off leader election
	heartbeatT  *time.Timer   //Ticker to triger heartbeat
	victory     chan bool     //Channel for victory in votting
	done        chan bool     //Done means candiate status changed to others
}

// return currentTerm and whether this server
// believes it is the leader.
func (rf *Raft) GetState() (int, bool) {
	rf.Lock()
	rf.Unlock()

	// Your code here (2A).
	if rf.status == Leader {
		return rf.currentTerm, true
	} else {
		return rf.currentTerm, false
	}
}

//
// save Raft's persistent state to stable storage,
// where it can later be retrieved after a crash and restart.
// see paper's Figure 2 for a description of what should be persistent.
//
func (rf *Raft) persist() {
	// Your code here (2C).
	// Example:
	// w := new(bytes.Buffer)
	// e := gob.NewEncoder(w)
	// e.Encode(rf.xxx)
	// e.Encode(rf.yyy)
	// data := w.Bytes()
	// rf.persister.SaveRaftState(data)
}

//
// restore previously persisted state.
//
func (rf *Raft) readPersist(data []byte) {
	// Your code here (2C).
	// Example:
	// r := bytes.NewBuffer(data)
	// d := gob.NewDecoder(r)
	// d.Decode(&rf.xxx)
	// d.Decode(&rf.yyy)
	if data == nil || len(data) < 1 { // bootstrap without any state?
		return
	}
}

//
// example RequestVote RPC arguments structure.
// field names must start with capital letters!
//
type RequestVoteArgs struct {
	// Your data here (2A, 2B).
	//LAB 2A
	Term         int //candidate term
	CandidateID  int //candidate who is requesting vote
	LastLogTerm  int //term of candidate's last log entry
	LastLogIndex int //index of candidate's last log entry
}

//
// example RequestVote RPC reply structure.
// field names must start with capital letters!
//
type RequestVoteReply struct {
	// Your data here (2A).
	Term        int  //currentTerm from voter and for candidate to update itself
	VoteGranted bool //true means candidate receive the vote
}

//
// example RequestAppend RPC arguments structure.
// field names must start with capital letters!
//
type RequestAppendArgs struct {
	// Your data here (2A, 2B).
	//LAB 2A
	Term     int     //leader's term
	LeaderID int     //leader's id,so follower can redirect clients
	Entries  []Entry //log entries to store(empty for heartbeat)
}

//
// example RequestAppend RPC reply structure.
// field names must start with capital letters!
//
type RequestAppendReply struct {
	// Your data here (2A).
	Term    int //currentTerm, for leader to update itself
	Success bool
	//true if follower contained entry matching
	//prevLogIndex and prevLogTerm
}

//
// Is candidate's log up to date ? (5.4.1)
//
func up2date(rf *Raft, candidate *RequestVoteArgs) bool {
	return true

	lastEntry := rf.log[len(rf.log)-1]

	//If the logs have last entries with different terms,
	//then the log with the later term is more up2date(5.4.1)
	if lastEntry.Term < candidate.LastLogTerm {
		return true
	}

	//The candidate's log end with the same term,
	//then whichever log is longer is more up2date
	if lastEntry.Term == candidate.LastLogTerm {
		lenLog := len(rf.log)
		if lenLog <= candidate.LastLogIndex+1 {
			return true
		} else {
			return false
		}
	}

	return false
}

//
// RequestVote RPC handler.
//
func (rf *Raft) RequestVote(candidate *RequestVoteArgs, reply *RequestVoteReply) {
	rf.Lock()
	defer rf.Unlock()
	fmt.Printf("RequestVote: %d:%d    request vote from .. ...%d:%d \n",
		rf.me, rf.currentTerm, candidate.CandidateID, candidate.Term)
	// Your code here (2A, 2B).
	//LAB 2A
	//Reply false if candidate.Term < rf.currentTerm (5.1)
	term := rf.currentTerm
	if candidate.Term < term {
		reply.Term = term
		reply.VoteGranted = false
		fmt.Printf("RequestVote: %d:%d>  reject from  ... %d:%d \n",
			rf.me, rf.currentTerm, candidate.CandidateID, candidate.Term)
		return
	}
	//LAB 2A
	//Each server will vote for at most one candidate in a given term (5.2)
	//When candidate's term is equal to this server's and votedFor is not null
	//these means the server have voted for in this term
	if candidate.Term == term {
		switch rf.status {
		case Candidate:
			reply.Term = term
			reply.VoteGranted = false
			fmt.Printf("RequestVote: %d:%dC=  voted myself reject ... %d:%d \n",
				rf.me, rf.currentTerm, candidate.CandidateID, candidate.Term)
			return
		case Leader:
			reply.Term = term
			reply.VoteGranted = false
			fmt.Printf("RequestVote: %d:%dL= I'm leader reject ... %d:%d \n",
				rf.me, rf.currentTerm, candidate.CandidateID, candidate.Term)
			return
		case Follower:
			if rf.votedFor != -1 {
				reply.Term = term
				reply.VoteGranted = false
				fmt.Printf("RequestVote: %d:%dF=  have voted for ... %d \n",
					rf.me, rf.currentTerm, rf.votedFor)
				return
			} else {
				if up2date(rf, candidate) {
					fmt.Printf("RequestVote: %d:%dF=  voted for  ... %d:%d \n",
						rf.me, rf.currentTerm, candidate.CandidateID, candidate.Term)
					rf.currentTerm = candidate.Term
					rf.votedFor = candidate.CandidateID
					reply.VoteGranted = true
					resetTimer(rf.timeoutT, timeOut())
					return
				} else {
					reply.Term = term
					reply.VoteGranted = false
					fmt.Printf("RequestVote: %d:%dF=  follow up2date than... %d:%d \n",
						rf.me, rf.currentTerm, candidate.CandidateID, candidate.Term)
					return
				}
			}
		}
	}

	//candidate.Term > term
	if rf.status == Candidate {
		rf.victory <- false
		<-rf.done
		fmt.Printf("RequestVote: %d:%d  Close election ... \n",
			rf.me, rf.currentTerm)
	}

	//LAB 2A
	//If votedFor is null or candidateID, and candidate's log is at least
	//as up-to-date as receiver's log, grant vote
	if up2date(rf, candidate) {
		//Candidate is at least as up-to-date as this server
		fmt.Printf("RequestVote: %d:%d<   less up-to-date  ... %d:%d \n",
			rf.me, rf.currentTerm, candidate.CandidateID, candidate.Term)
		switch rf.status {
		case Candidate:
			fmt.Printf("RequestVote: %d:%dC<   Warning should not happen ... %d:%d \n",
				rf.me, rf.currentTerm, candidate.CandidateID, candidate.Term)
		case Leader:
			l2f(rf)
			fmt.Printf("RequestVote: %d:%dL<   leader to follow \n",
				rf.me, rf.currentTerm)
		case Follower:
			resetTimer(rf.timeoutT, timeOut())
			break
		}

		fmt.Printf("RequestVote: %d:%d<   voting for     ... %d:%d \n",
			rf.me, rf.currentTerm, candidate.CandidateID, candidate.Term)
		rf.currentTerm = candidate.Term
		rf.votedFor = candidate.CandidateID
		reply.VoteGranted = true
	} else {
		fmt.Printf("RequestVote: %d:%d<   more up2date %d:%d \n",
			rf.me, rf.currentTerm, candidate.CandidateID, candidate.Term)
		reply.Term = term
		reply.VoteGranted = false
	}
	return
}

//
// example code to send a RequestVote RPC to a server.
// server is the index of the target server in rf.peers[].
// expects RPC arguments in args.
// fills in *reply with RPC reply, so caller should
// pass &reply.
// the types of the args and reply passed to Call() must be
// the same as the types of the arguments declared in the
// handler function (including whether they are pointers).
//
// The labrpc package simulates a lossy network, in which servers
// may be unreachable, and in which requests and replies may be lost.
// Call() sends a request and waits for a reply. If a reply arrives
// within a timeout interval, Call() returns true; otherwise
// Call() returns false. Thus Call() may not return for a while.
// A false return can be caused by a dead server, a live server that
// can't be reached, a lost request, or a lost reply.
//
// Call() is guaranteed to return (perhaps after a delay) *except* if the
// handler function on the server side does not return.  Thus there
// is no need to implement your own timeouts around Call().
//
// look at the comments in ../labrpc/labrpc.go for more details.
//
// if you're having trouble getting RPC to work, check that you've
// capitalized all field names in structs passed over RPC, and
// that the caller passes the address of the reply struct with &, not
// the struct itself.
//
func (rf *Raft) sendRequestVote(server int, args *RequestVoteArgs, reply *RequestVoteReply) bool {
	ok := rf.peers[server].Call("Raft.RequestVote", args, reply)
	return ok
}

//
// RequestAppend RPC handler.
//
func (rf *Raft) RequestAppend(leader *RequestAppendArgs, reply *RequestAppendReply) {
	rf.Lock()
	defer rf.Unlock()
	//fmt.Printf("RequestAppend: %d:%d  request from  ... %d:%d \n",
	//	rf.me, rf.currentTerm, leader.LeaderID, leader.Term)
	//Your code here (2A, 2B).
	//LAB 2A
	//Reply false if args.Term < rf.currentTerm (5.1)
	if term := rf.currentTerm; leader.Term < term {
		fmt.Printf("RequestAppend: %d:%d>  reject from  ... %d:%d \n",
			rf.me, rf.currentTerm, leader.LeaderID, leader.Term)
		reply.Term = term
		reply.Success = false
		return
	}
	//LAB 2A
	//leader.Term >= rf.currentTerm means leader is more up-to-date
	//than this server and the server should return to follower
	if leader.Term == rf.currentTerm {
		switch rf.status {
		case Candidate:
			//new leader elected when you in candidate
			rf.victory <- false
			<-rf.done
			fmt.Printf("RequestAppend: %d:%dC= another leader elected & change status to follow  %d:%d \n",
				rf.me, rf.currentTerm, leader.LeaderID, leader.Term)
			//resetTimer and append success
		case Leader:
			fmt.Printf("Raft.RequestAppend: 1.Two leaders exist with same term!!! \n")
		case Follower:
			//resetTimer and append success
			fmt.Printf("RequestAppend: %d:%dF= get heartbeat from %d:%d \n",
				rf.me, rf.currentTerm, leader.LeaderID, leader.Term)
			resetTimer(rf.timeoutT, timeOut())
			break
		}

		if len(leader.Entries) == 0 {
			rf.currentTerm = leader.Term
			reply.Success = true
		} else {
			fmt.Printf("Raft.RequestAppend: Warning Append with logs\n")
		}
		return
	}

	//Leader.Term > rf.currentTerm
	switch rf.status {
	case Candidate:
		//leader.Term >  rf.currentTerm means the raft should be follow ASAP
		rf.victory <- false
		<-rf.done
		fmt.Printf("RequestAppend: %d:%dC< get heartbeat from %d:%d \n",
			rf.me, rf.currentTerm, leader.LeaderID, leader.Term)
		//resetTimer and append success
	case Leader:
		//leader.Term >  rf.currentTerm this will not occur
		fmt.Printf("Raft.RequestAppend: Two leaders exist!!! The old leader should change to follower\n")
		l2f(rf)
	case Follower:
		fmt.Printf("RequestAppend: %d:%dF< get heartbeat from %d:%d \n",
			rf.me, rf.currentTerm, leader.LeaderID, leader.Term)
		resetTimer(rf.timeoutT, timeOut())
		break
	}
	//Heartbeats - Append RPC with no log entry
	//Reset this follow's timer

	if len(leader.Entries) == 0 {
		rf.currentTerm = leader.Term
		reply.Success = true
	} else {
		fmt.Printf("Raft.RequestAppend: Warning Append with logs\n")
	}
	return

}

//
// example code to send a RequestAppend RPC to a server.
//
func (rf *Raft) sendRequestAppend(server int, args *RequestAppendArgs, reply *RequestAppendReply) bool {
	ok := rf.peers[server].Call("Raft.RequestAppend", args, reply)
	return ok
}

//
// the service using Raft (e.g. a k/v server) wants to start
// agreement on the next command to be appended to Raft's log. if this
// server isn't the leader, returns false. otherwise start the
// agreement and return immediately. there is no guarantee that this
// command will ever be committed to the Raft log, since the leader
// may fail or lose an election.
//
// the first return value is the index that the command will appear at
// if it's ever committed. the second return value is the current
// term. the third return value is true if this server believes it is
// the leader.
//
func (rf *Raft) Start(command interface{}) (int, int, bool) {
	index := -1
	term := -1
	isLeader := true

	// Your code here (2B).

	return index, term, isLeader
}

//
// the tester calls Kill() when a Raft instance won't
// be needed again. you are not required to do anything
// in Kill(), but it might be convenient to (for example)
// turn off debug output from this instance.
//
func (rf *Raft) Kill() {
	// Your code here, if desired.
}

//
// Return the duration of timeout and avoid the split vote
// by randmon select timeout in the range of 150 to 300
// millisecond (5.2)
//
const (
	TimeFrom   = 100
	TimeTo     = 200
	HeartBeats = 10
	Leader     = 0
	Candidate  = 1
	Follower   = 2
)

func timeOut() time.Duration {
	rand.Seed(time.Now().UTC().UnixNano())
	randNum := rand.Intn(TimeTo-TimeFrom) + TimeFrom
	//duration := randNum * time.Millisecond
	//fmt.Println(randNum)
	return time.Duration(randNum) * time.Millisecond
}

//
//After duration expires, the leader election startup
//
func election(rf *Raft) {

	rf.Lock()
	//Stop the timer firstly
	rf.status = Candidate
	//To begin an election, a follower increments its current
	//term and transitions to candidate state. It then votes for
	//itself (5.2)
	rf.currentTerm = rf.currentTerm + 1
	currentTerm := rf.currentTerm
	resetTimer(rf.timeoutT, timeOut())
	me := rf.me
	rf.votedFor = rf.me
	votedNum := 1
	againstNum := 0
	//drain victory channel because victory will set to false many times
	//when a raftA is candidate and it get reject vote from raftB with bigger term
	//at the same time a leader(raftC) have been selected and send heartbeat to raftA
	drainChan(rf.victory)
	drainChan(rf.done)

	//candidate ended means candidate have changed status no mater to leader or follow
	cdtEnded := false

	// Lock to protect for votedNumber
	voteMu := &sync.Mutex{}
	againstMu := &sync.Mutex{}

	//Issues RequestVote RPCs in parallel to each of
	//the other servers in the cluster.(5.2)
	args := &RequestVoteArgs{}
	args.Term = rf.currentTerm
	args.CandidateID = rf.me
	args.LastLogTerm = rf.log[len(rf.log)-1].Term
	args.LastLogIndex = len(rf.log) - 1

	replys := make([]RequestVoteReply, len(rf.peers))
	peers := rf.peers
	fmt.Printf("F2C electing  .............%d:%d\n", me, rf.currentTerm)
	rf.Unlock()

	for idx := 0; idx < len(peers); idx++ {
		if idx == me {
			continue
		}
		go func(idx int) {
		ReVote:
			if cdtEnded {
				return
			}
			taskState := peers[idx].Call("Raft.RequestVote", args, &replys[idx])
			//Servers retry RPCs if they do not receive a response
			//in a timely manner(5.1 last line).
			//This means retry after the rpc timeout interval ,
			//so no need to implement your own timeouts around Call

			//If a follower of or candidate creashed, the future RequestVote
			//and AppendEntries PRC sent to it will fail. Raft handles these
			//failurs by retrying indefinitely.(5.5)
			if taskState == false {
				//fmt.Printf("Raft.Election:RPC error - %v send to %v\n", me, idx)
				goto ReVote
			}
			if replys[idx].VoteGranted == true {
				voteMu.Lock()
				votedNum++
				if votedNum >= (len(peers)/2 + 1) {
					rf.victory <- true
				}
				voteMu.Unlock()
			} else {
				//1.VoteGranted is false and reply's term is equal to the currentTerm,
				//  it means replyer have voted for another candidate
				againstMu.Lock()
				againstNum++
				if againstNum >= (len(peers)/2 + len(peers)%2) {
					rf.victory <- false
				}
				againstMu.Unlock()

				//2.VoteGranted is false and reply's term is big than currentTerm,
				//  it means replyer is more up-to-date than caller,
				//  replyer take the vote power and the candidate should be back to
				//  follower immediately
				if replys[idx].Term > currentTerm {
					rf.victory <- false
				}
			}
			return
		}(idx)
	} //end for

	if <-rf.victory {
		c2l(rf)
	} else {
		c2f(rf)
	}
	cdtEnded = true
	go func() { rf.done <- true }()
	return
}

//
//Reset timer
//
func stopTimer(t *time.Timer) {
	if !t.Stop() {
		select {
		case <-t.C:
			fmt.Println("resetTimer: What happend??")
			fmt.Println("resetTimer: This means timeout happend but no electioin trigged!")
		default:
		}
	}
}

func resetTimer(t *time.Timer, d time.Duration) {
	//fmt.Println("Stop & reset timer.............")
	stopTimer(t)
	//t = time.NewTimer(time.Duration(30) * time.Millisecond)
	t.Reset(d)
}

//Do not retry if the RequestAppend RPC do not receive a response
func beatOnce(rf *Raft) {
	//Args for heartbeat
	args := &RequestAppendArgs{}
	args.Term = rf.currentTerm
	args.LeaderID = rf.me
	me := rf.me
	args.Entries = []Entry{}

	//Prepare for all the replys
	replys := make([]RequestAppendReply, len(rf.peers))

	peers := rf.peers
	for idx := 0; idx < len(rf.peers); idx++ {
		if idx == me {
			continue
		}
		go func(idx int) {
		Rebeat:
			taskState := peers[idx].Call("Raft.RequestAppend", args, &replys[idx])
			//Servers retry RPCs if they do not receive a response
			//in a timely manner(5.1 last line).
			//This means retry after the rpc timeout interval ,
			//so no need to implement your own timeouts around Call

			//If a follower of or candidate creashed, the future RequestVote
			//and AppendEntries PRC sent to it will fail. Raft handles these
			//failurs by retrying indefinitely.(5.5)
			if taskState == false {
				//fmt.Printf("Raft.Heartbeat:RPC error from %d to %d\n", me, idx)
				goto Rebeat
			}
			return
		}(idx)
	} //end for
}

func c2l(rf *Raft) {
	fmt.Println("C2L          .............", rf.me)
	//candidate to leader
	rf.status = Leader
	rf.votedFor = -1
	//Stop the timeout timer
	stopTimer(rf.timeoutT)
	//Send the heart beat to all the others
	heartbeatD := time.Duration(HeartBeats) * time.Millisecond
	rf.heartbeatT = time.NewTimer(heartbeatD)
	go func() {
		for {
			select {
			case <-rf.heartbeatT.C:
				beatOnce(rf)
				//resetTimer(rf.heartbeatT, heartbeatD)
				rf.heartbeatT.Reset(heartbeatD)
			}

		}
	}()
}

func c2f(rf *Raft) {
	//candidate to follower
	rf.status = Follower
	rf.currentTerm = rf.currentTerm - 1
	rf.votedFor = -1
	resetTimer(rf.timeoutT, timeOut())

	//rf.timeoutT = time.NewTimer(time.Duration(30) * time.Millisecond)
	fmt.Println("C2F          .............", rf.me)
}

func l2f(rf *Raft) {
	//Leader to follower
	rf.status = Follower
	rf.votedFor = -1
	//stop the heartbeat
	stopTimer(rf.heartbeatT)
	//reset timeout timer
	resetTimer(rf.timeoutT, timeOut())
	fmt.Println("L2F          .............", rf.me)
}

func drainChan(c chan bool) {
	for {
		select {
		case <-c:
			fmt.Println("drain the chan .............")
			continue
		default:
			return
		}
	}
}

//
// the service or tester wants to create a Raft server. the ports
// of all the Raft servers (including this one) are in peers[]. this
// server's port is peers[me]. all the servers' peers[] arrays
// have the same order. persister is a place for this server to
// save its persistent state, and also initially holds the most
// recent saved state, if any. applyCh is a channel on which the
// tester or service expects Raft to send ApplyMsg messages.
// Make() must return quickly, so it should start goroutines
// for any long-running work.
//
func Make(peers []*labrpc.ClientEnd, me int,
	persister *Persister, applyCh chan ApplyMsg) *Raft {
	rf := &Raft{}
	rf.currentTerm = 0
	rf.status = Follower
	rf.peers = peers
	rf.persister = persister
	rf.me = me
	rf.votedFor = -1
	//rf.timeoutT = time.NewTimer(timeOut())
	//array := [3]int{188, 176, 179}
	array := [3]int{149, 149, 149}
	rf.timeoutT = time.NewTimer(time.Duration(array[me]) * time.Millisecond)
	rf.victory = make(chan bool)
	rf.done = make(chan bool)
	rf.log = make(map[int]Entry)

	// Your initialization code here (2A, 2B, 2C).
	//LAB 2A
	go func() {
		//Kick off leader election periodically
		for {
			select {
			case <-applyCh:
				fmt.Println("Receive a apply from leader")
			case <-rf.timeoutT.C:
				fmt.Printf("%d timeout     .............\n", rf.me)
				election(rf)
			default:
			}
		}
	}()

	// initialize from state persisted before a crash
	rf.readPersist(persister.ReadRaftState())

	return rf
}
