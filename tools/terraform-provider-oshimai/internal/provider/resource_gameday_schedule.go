package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// gameDayScheduleResource manages one recurring schedule via Oshimai's
// POST/GET/DELETE/{id}/toggle /api/v1/gameday/schedules endpoints.
//
// The server has no GET-by-id or PATCH route: Read lists every schedule and matches by id, and
// only "enabled" can change in place via the toggle endpoint — every other attribute change
// replaces the resource (delete the old schedule, create a new one), enforced below with
// RequiresReplace plan modifiers.
type gameDayScheduleResource struct {
	client *client
}

type gameDayScheduleModel struct {
	ID         types.String `tfsdk:"id"`
	Name       types.String `tfsdk:"name"`
	Weekday    types.Int64  `tfsdk:"weekday"`
	HourUTC    types.Int64  `tfsdk:"hour_utc"`
	MinuteUTC  types.Int64  `tfsdk:"minute_utc"`
	Enabled    types.Bool   `tfsdk:"enabled"`
	RunRequest types.String `tfsdk:"run_request_json"`
}

func NewGameDayScheduleResource() resource.Resource {
	return &gameDayScheduleResource{}
}

func (r *gameDayScheduleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_gameday_schedule"
}

func (r *gameDayScheduleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A recurring GameDay chaos/load run on an Oshimai server, e.g. \"every Tuesday at 09:00 UTC, run the checkout resilience scenario.\"",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "Server-assigned schedule id, e.g. gameday-1.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required:      true,
				Description:   "Human-readable schedule name.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"weekday": schema.Int64Attribute{
				Required:    true,
				Description: "0=Sunday .. 6=Saturday.",
			},
			"hour_utc": schema.Int64Attribute{
				Required:    true,
				Description: "Hour of day in UTC, 0-23.",
			},
			"minute_utc": schema.Int64Attribute{
				Required:    true,
				Description: "Minute of the hour in UTC, 0-59.",
			},
			"enabled": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Whether the schedule fires. Defaults to true on creation; can be toggled without replacing the resource.",
			},
			"run_request_json": schema.StringAttribute{
				Required:      true,
				Description:   "The run to fire, as the JSON body accepted by POST /api/v1/runs (jsonencode({...}) in Terraform).",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
		},
	}
}

func (r *gameDayScheduleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data type", fmt.Sprintf("got %T", req.ProviderData))
		return
	}
	r.client = c
}

type scheduleWire struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Weekday     int             `json:"weekday"`
	HourUTC     int             `json:"hour_utc"`
	MinuteUTC   int             `json:"minute_utc"`
	Enabled     bool            `json:"enabled"`
	Payload     json.RawMessage `json:"payload"`
	LastFiredAt string          `json:"last_fired_at,omitempty"`
}

func (r *gameDayScheduleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan gameDayScheduleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var runPayload json.RawMessage
	if err := json.Unmarshal([]byte(plan.RunRequest.ValueString()), &runPayload); err != nil {
		resp.Diagnostics.AddError("Invalid run_request_json", err.Error())
		return
	}

	body, _ := json.Marshal(map[string]any{
		"name":       plan.Name.ValueString(),
		"weekday":    plan.Weekday.ValueInt64(),
		"hour_utc":   plan.HourUTC.ValueInt64(),
		"minute_utc": plan.MinuteUTC.ValueInt64(),
		"run":        runPayload,
	})

	var created struct {
		ID string `json:"id"`
	}
	if err := r.client.doJSON(ctx, http.MethodPost, "/api/v1/gameday/schedules", body, &created); err != nil {
		resp.Diagnostics.AddError("Error creating GameDay schedule", err.Error())
		return
	}

	plan.ID = types.StringValue(created.ID)
	plan.Enabled = types.BoolValue(true)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *gameDayScheduleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state gameDayScheduleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var all []scheduleWire
	if err := r.client.doJSON(ctx, http.MethodGet, "/api/v1/gameday/schedules", nil, &all); err != nil {
		resp.Diagnostics.AddError("Error listing GameDay schedules", err.Error())
		return
	}

	var found *scheduleWire
	for i := range all {
		if all[i].ID == state.ID.ValueString() {
			found = &all[i]
			break
		}
	}
	if found == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	state.Name = types.StringValue(found.Name)
	state.Weekday = types.Int64Value(int64(found.Weekday))
	state.HourUTC = types.Int64Value(int64(found.HourUTC))
	state.MinuteUTC = types.Int64Value(int64(found.MinuteUTC))
	state.Enabled = types.BoolValue(found.Enabled)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *gameDayScheduleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state gameDayScheduleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Every other attribute is RequiresReplace, so the only in-place update possible is "enabled".
	if plan.Enabled.ValueBool() != state.Enabled.ValueBool() {
		body, _ := json.Marshal(map[string]any{"enabled": plan.Enabled.ValueBool()})
		path := fmt.Sprintf("/api/v1/gameday/schedules/%s/toggle", state.ID.ValueString())
		if err := r.client.doJSON(ctx, http.MethodPost, path, body, nil); err != nil {
			resp.Diagnostics.AddError("Error toggling GameDay schedule", err.Error())
			return
		}
	}

	plan.ID = state.ID
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *gameDayScheduleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state gameDayScheduleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	path := fmt.Sprintf("/api/v1/gameday/schedules/%s", state.ID.ValueString())
	if err := r.client.doJSON(ctx, http.MethodDelete, path, nil, nil); err != nil {
		resp.Diagnostics.AddError("Error deleting GameDay schedule", err.Error())
	}
}

// doJSON is the shared HTTP helper: sends body (if any) as JSON, decodes a JSON response into out
// (if non-nil), and turns any non-2xx status into an error carrying the response body.
func (c *client) doJSON(ctx context.Context, method, path string, body []byte, out any) error {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("oshimai server returned %d: %s", resp.StatusCode, string(respBody))
	}
	if out != nil && len(respBody) > 0 {
		return json.Unmarshal(respBody, out)
	}
	return nil
}
