package subsidy

import (
	"strconv"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

type CodeType = sdk.CodeType

const (
	DefaultCodespace sdk.CodespaceType = "SUBSIDY"

	// Simple Gov errors reserve 700 ~ 799.
	CodeInvalidOption         CodeType = 1
	CodeInvalidProposalID     CodeType = 2
	CodeVotingPeriodClosed    CodeType = 3
	CodeEmptyProposalQueue    CodeType = 4
	CodeInvalidTitle          CodeType = 5
	CodeInvalidDescription    CodeType = 6
	CodeInvalidVotingWindow   CodeType = 7
	CodeProposalNotFound      CodeType = 8
	CodeVoteNotFound          CodeType = 9
	CodeProposalQueueNotFound CodeType = 10
	CodeInvalidDeposit        CodeType = 11
)

func codeToDefaultMsg(code CodeType) string {
	switch code {
	case CodeInvalidOption:
		return "Invalid option"
	case CodeInvalidProposalID:
		return "Invalid proposalID"
	case CodeVotingPeriodClosed:
		return "Voting Period Closed"
	case CodeEmptyProposalQueue:
		return "ProposalQueue is empty"
	case CodeInvalidTitle:
		return "Invalid proposal title"
	case CodeInvalidDescription:
		return "Invalid proposal description"
	case CodeInvalidVotingWindow:
		return "Invalid voting window"
	case CodeProposalNotFound:
		return "Proposal not found"
	case CodeVoteNotFound:
		return "Option not found"
	case CodeProposalQueueNotFound:
		return "Proposal Queue not found"
	case CodeInvalidDeposit:
		return "Invalid deposit"
	default:
		return sdk.CodeToDefaultMsg(code)
	}
}

//----------------------------------------
// Error constructors

// nolint
func ErrInvalidOption(msg string) sdk.Error {
	if msg != "" {
		return newError(DefaultCodespace, CodeInvalidOption, msg)
	}
	return newError(DefaultCodespace, CodeInvalidOption, "The chosen option is invalid")
}

// nolint
func ErrInvalidProposalID(msg string) sdk.Error {
	if msg != "" {
		return newError(DefaultCodespace, CodeInvalidProposalID, msg)
	}
	return newError(DefaultCodespace, CodeInvalidProposalID, "ProposalID is not valid")
}

// nolint
func ErrInvalidTitle() sdk.Error {
	return newError(DefaultCodespace, CodeInvalidTitle, "Cannot submit a proposal with empty title")
}

// nolint
func ErrInvalidDescription() sdk.Error {
	return newError(DefaultCodespace, CodeInvalidDescription, "Cannot submit a proposal with empty description")
}

// nolint
func ErrVotingPeriodClosed() sdk.Error {
	return newError(DefaultCodespace, CodeVotingPeriodClosed, "Voting period is closed for this proposal")
}

// nolint
func ErrEmptyProposalQueue() sdk.Error {
	return newError(DefaultCodespace, CodeEmptyProposalQueue, "Can't get element from an empty proposal queue")
}

// nolint
func ErrProposalNotFound(proposalID int64) sdk.Error {
	return newError(DefaultCodespace, CodeProposalNotFound, "Proposal with id "+
		strconv.Itoa(int(proposalID))+" not found")
}

// nolint
func ErrVoteNotFound() sdk.Error {
	return newError(DefaultCodespace, CodeVoteNotFound, "Vote not found")
}

// nolint
func ErrProposalQueueNotFound() sdk.Error {
	return newError(DefaultCodespace, CodeProposalQueueNotFound, "Proposal Queue not found")
}

// nolint
func ErrInvalidVotingWindow(msg string) sdk.Error {
	if msg != "" {
		return newError(DefaultCodespace, CodeInvalidVotingWindow, msg)
	}
	return newError(DefaultCodespace, CodeInvalidVotingWindow, "Voting window is not positive")
}

// nolint
func ErrMinimumDeposit() sdk.Error {
	return newError(DefaultCodespace, CodeInvalidDeposit, "Deposit is lower than the minimum")
}

//----------------------------------------

func msgOrDefaultMsg(msg string, code CodeType) string {
	if msg != "" {
		return msg
	}
	return codeToDefaultMsg(code)
}

func newError(codespace sdk.CodespaceType, code CodeType, msg string) sdk.Error {
	msg = msgOrDefaultMsg(msg, code)
	return sdk.NewError(codespace, code, msg)
}
