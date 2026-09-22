package server

// Operations the hosted service had and this server does not: billing, teams,
// invitations, shares, analytics and feedback. Each answers 501 with a message
// naming the operation, so the CLI prints something a person can act on rather
// than a decoding error. Generated from api.StrictServerInterface; regenerate
// by hand when the spec changes.

import (
	"context"
	"net/http"

	"github.com/timescale/ghost/internal/api"
)

func (s *Server) AnalyticsIdentify(context.Context, api.AnalyticsIdentifyRequestObject) (api.AnalyticsIdentifyResponseObject, error) {
	return api.AnalyticsIdentifydefaultJSONResponse{Body: unsupported("AnalyticsIdentify"), StatusCode: http.StatusNotImplemented}, nil
}

func (s *Server) AnalyticsTrack(context.Context, api.AnalyticsTrackRequestObject) (api.AnalyticsTrackResponseObject, error) {
	return api.AnalyticsTrackdefaultJSONResponse{Body: unsupported("AnalyticsTrack"), StatusCode: http.StatusNotImplemented}, nil
}

func (s *Server) Logout(context.Context, api.LogoutRequestObject) (api.LogoutResponseObject, error) {
	return api.LogoutdefaultJSONResponse{Body: unsupported("Logout"), StatusCode: http.StatusNotImplemented}, nil
}

func (s *Server) SubmitFeedback(context.Context, api.SubmitFeedbackRequestObject) (api.SubmitFeedbackResponseObject, error) {
	return api.SubmitFeedbackdefaultJSONResponse{Body: unsupported("SubmitFeedback"), StatusCode: http.StatusNotImplemented}, nil
}

func (s *Server) ListReceivedInvites(context.Context, api.ListReceivedInvitesRequestObject) (api.ListReceivedInvitesResponseObject, error) {
	return api.ListReceivedInvitesdefaultJSONResponse{Body: unsupported("ListReceivedInvites"), StatusCode: http.StatusNotImplemented}, nil
}

func (s *Server) AcceptInvite(context.Context, api.AcceptInviteRequestObject) (api.AcceptInviteResponseObject, error) {
	return api.AcceptInvitedefaultJSONResponse{Body: unsupported("AcceptInvite"), StatusCode: http.StatusNotImplemented}, nil
}

func (s *Server) DeclineInvite(context.Context, api.DeclineInviteRequestObject) (api.DeclineInviteResponseObject, error) {
	return api.DeclineInvitedefaultJSONResponse{Body: unsupported("DeclineInvite"), StatusCode: http.StatusNotImplemented}, nil
}

func (s *Server) GetPricing(context.Context, api.GetPricingRequestObject) (api.GetPricingResponseObject, error) {
	return api.GetPricingdefaultJSONResponse{Body: unsupported("GetPricing"), StatusCode: http.StatusNotImplemented}, nil
}

func (s *Server) CreateSpace(context.Context, api.CreateSpaceRequestObject) (api.CreateSpaceResponseObject, error) {
	return api.CreateSpacedefaultJSONResponse{Body: unsupported("CreateSpace"), StatusCode: http.StatusNotImplemented}, nil
}

func (s *Server) CreateAPIKey(context.Context, api.CreateAPIKeyRequestObject) (api.CreateAPIKeyResponseObject, error) {
	return api.CreateAPIKeydefaultJSONResponse{Body: unsupported("CreateAPIKey"), StatusCode: http.StatusNotImplemented}, nil
}

func (s *Server) DeleteAPIKey(context.Context, api.DeleteAPIKeyRequestObject) (api.DeleteAPIKeyResponseObject, error) {
	return api.DeleteAPIKeydefaultJSONResponse{Body: unsupported("DeleteAPIKey"), StatusCode: http.StatusNotImplemented}, nil
}

func (s *Server) ShareDatabase(context.Context, api.ShareDatabaseRequestObject) (api.ShareDatabaseResponseObject, error) {
	return api.ShareDatabasedefaultJSONResponse{Body: unsupported("ShareDatabase"), StatusCode: http.StatusNotImplemented}, nil
}

func (s *Server) ListInvites(context.Context, api.ListInvitesRequestObject) (api.ListInvitesResponseObject, error) {
	return api.ListInvitesdefaultJSONResponse{Body: unsupported("ListInvites"), StatusCode: http.StatusNotImplemented}, nil
}

func (s *Server) CreateInvite(context.Context, api.CreateInviteRequestObject) (api.CreateInviteResponseObject, error) {
	return api.CreateInvitedefaultJSONResponse{Body: unsupported("CreateInvite"), StatusCode: http.StatusNotImplemented}, nil
}

func (s *Server) CancelInvite(context.Context, api.CancelInviteRequestObject) (api.CancelInviteResponseObject, error) {
	return api.CancelInvitedefaultJSONResponse{Body: unsupported("CancelInvite"), StatusCode: http.StatusNotImplemented}, nil
}

func (s *Server) ListInvoices(context.Context, api.ListInvoicesRequestObject) (api.ListInvoicesResponseObject, error) {
	return api.ListInvoicesdefaultJSONResponse{Body: unsupported("ListInvoices"), StatusCode: http.StatusNotImplemented}, nil
}

func (s *Server) GetInvoice(context.Context, api.GetInvoiceRequestObject) (api.GetInvoiceResponseObject, error) {
	return api.GetInvoicedefaultJSONResponse{Body: unsupported("GetInvoice"), StatusCode: http.StatusNotImplemented}, nil
}

func (s *Server) LeaveSpace(context.Context, api.LeaveSpaceRequestObject) (api.LeaveSpaceResponseObject, error) {
	return api.LeaveSpacedefaultJSONResponse{Body: unsupported("LeaveSpace"), StatusCode: http.StatusNotImplemented}, nil
}

func (s *Server) ListMembers(context.Context, api.ListMembersRequestObject) (api.ListMembersResponseObject, error) {
	return api.ListMembersdefaultJSONResponse{Body: unsupported("ListMembers"), StatusCode: http.StatusNotImplemented}, nil
}

func (s *Server) RemoveMember(context.Context, api.RemoveMemberRequestObject) (api.RemoveMemberResponseObject, error) {
	return api.RemoveMemberdefaultJSONResponse{Body: unsupported("RemoveMember"), StatusCode: http.StatusNotImplemented}, nil
}

func (s *Server) UpdateMemberRole(context.Context, api.UpdateMemberRoleRequestObject) (api.UpdateMemberRoleResponseObject, error) {
	return api.UpdateMemberRoledefaultJSONResponse{Body: unsupported("UpdateMemberRole"), StatusCode: http.StatusNotImplemented}, nil
}

func (s *Server) UpdateOverages(context.Context, api.UpdateOveragesRequestObject) (api.UpdateOveragesResponseObject, error) {
	return api.UpdateOveragesdefaultJSONResponse{Body: unsupported("UpdateOverages"), StatusCode: http.StatusNotImplemented}, nil
}

func (s *Server) ListPaymentMethods(context.Context, api.ListPaymentMethodsRequestObject) (api.ListPaymentMethodsResponseObject, error) {
	return api.ListPaymentMethodsdefaultJSONResponse{Body: unsupported("ListPaymentMethods"), StatusCode: http.StatusNotImplemented}, nil
}

func (s *Server) CreatePaymentMethodSetup(context.Context, api.CreatePaymentMethodSetupRequestObject) (api.CreatePaymentMethodSetupResponseObject, error) {
	return api.CreatePaymentMethodSetupdefaultJSONResponse{Body: unsupported("CreatePaymentMethodSetup"), StatusCode: http.StatusNotImplemented}, nil
}

func (s *Server) DeletePaymentMethod(context.Context, api.DeletePaymentMethodRequestObject) (api.DeletePaymentMethodResponseObject, error) {
	return api.DeletePaymentMethoddefaultJSONResponse{Body: unsupported("DeletePaymentMethod"), StatusCode: http.StatusNotImplemented}, nil
}

func (s *Server) GetPaymentMethod(context.Context, api.GetPaymentMethodRequestObject) (api.GetPaymentMethodResponseObject, error) {
	return api.GetPaymentMethoddefaultJSONResponse{Body: unsupported("GetPaymentMethod"), StatusCode: http.StatusNotImplemented}, nil
}

func (s *Server) CancelPaymentMethodDeletion(context.Context, api.CancelPaymentMethodDeletionRequestObject) (api.CancelPaymentMethodDeletionResponseObject, error) {
	return api.CancelPaymentMethodDeletiondefaultJSONResponse{Body: unsupported("CancelPaymentMethodDeletion"), StatusCode: http.StatusNotImplemented}, nil
}

func (s *Server) SetPaymentMethodPrimary(context.Context, api.SetPaymentMethodPrimaryRequestObject) (api.SetPaymentMethodPrimaryResponseObject, error) {
	return api.SetPaymentMethodPrimarydefaultJSONResponse{Body: unsupported("SetPaymentMethodPrimary"), StatusCode: http.StatusNotImplemented}, nil
}

func (s *Server) RenameSpace(context.Context, api.RenameSpaceRequestObject) (api.RenameSpaceResponseObject, error) {
	return api.RenameSpacedefaultJSONResponse{Body: unsupported("RenameSpace"), StatusCode: http.StatusNotImplemented}, nil
}

func (s *Server) ListShares(context.Context, api.ListSharesRequestObject) (api.ListSharesResponseObject, error) {
	return api.ListSharesdefaultJSONResponse{Body: unsupported("ListShares"), StatusCode: http.StatusNotImplemented}, nil
}

func (s *Server) RevokeShare(context.Context, api.RevokeShareRequestObject) (api.RevokeShareResponseObject, error) {
	return api.RevokeSharedefaultJSONResponse{Body: unsupported("RevokeShare"), StatusCode: http.StatusNotImplemented}, nil
}
