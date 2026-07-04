package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/arturoburigo/gzlab/internal/gitlab"
)

func TestPrintMRList(t *testing.T) {
	var buf bytes.Buffer
	mrs := []*gitlab.MergeRequest{
		{IID: 1, Title: "First MR"},
		{IID: 2, Title: "Second MR", Draft: true},
	}
	if err := printMRList(&buf, mrs); err != nil {
		t.Fatalf("printMRList: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"!1", "First MR", "!2", "Second MR", "(draft)"} {
		if !strings.Contains(out, want) {
			t.Errorf("printMRList missing %q\n%s", want, out)
		}
	}
}

func TestPrintMRList_Empty(t *testing.T) {
	var buf bytes.Buffer
	if err := printMRList(&buf, nil); err != nil {
		t.Fatalf("printMRList: %v", err)
	}
	if !strings.Contains(buf.String(), "No open merge requests") {
		t.Errorf("empty list output = %q", buf.String())
	}
}

func TestFormatMRDetail(t *testing.T) {
	mr := &gitlab.MergeRequest{
		IID: 251, Title: "Alinha ao commons",
		SourceBranch: "bugfix-PD-26527", TargetBranch: "master",
		Author: "arturo.burigo", State: gitlab.MergeRequestStateOpened,
		ApprovalsRequired: 2, ApprovalsGiven: 1, HasConflicts: true,
		Pipeline: &gitlab.Pipeline{Status: gitlab.PipelineStatusFailed},
		WebURL:   "https://gitlab.services.betha.cloud/x/-/merge_requests/251",
	}
	got := formatMRDetail(mr)
	for _, want := range []string{
		"!251 Alinha ao commons",
		"bugfix-PD-26527 -> master",
		"arturo.burigo",
		"opened",
		"failed",
		"1/2",
		"Conflicts: yes",
		"https://gitlab.services.betha.cloud/x/-/merge_requests/251",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("formatMRDetail missing %q\n%s", want, got)
		}
	}
}

func TestFormatMRDetail_OmitsEmptyFields(t *testing.T) {
	got := formatMRDetail(&gitlab.MergeRequest{IID: 7, Title: "Minimal", SourceBranch: "a", TargetBranch: "b"})
	for _, absent := range []string{"Author:", "Pipeline:", "Approvals:", "Conflicts:"} {
		if strings.Contains(got, absent) {
			t.Errorf("formatMRDetail should omit %q\n%s", absent, got)
		}
	}
}
