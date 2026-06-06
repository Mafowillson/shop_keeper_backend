package i18n

// Messages holds every user-facing string the backend can return.
// Add new fields here; never hardcode strings in handlers.
type Messages struct {
	// ── Auth ─────────────────────────────────────────────────────────────────
	InvalidCredentials string
	Unauthorized       string
	TokenExpired       string
	AccountDeactivated string
	ForbiddenRole      string
	PasswordTooShort   string
	InvalidEmail       string
	MissingAuthToken   string
	InvalidTokenFormat string
	InvalidToken       string
	BearerRequired     string

	// ── General ──────────────────────────────────────────────────────────────
	NotFound             string
	InternalError        string
	ValidationError      string
	Success              string
	Created              string
	Updated              string
	Deleted              string
	InvalidID            string
	MissingRequiredField string
	BadRequest           string
	InvalidJSON          string

	// ── Products ─────────────────────────────────────────────────────────────
	ProductNotFound          string
	ProductCreated           string
	ProductUpdated           string
	ProductDeleted           string
	InsufficientStock        string
	InvalidPrice             string
	InvalidStockQuantity     string
	ProductNameRequired      string
	LowStockThresholdInvalid string

	// ── Sales ────────────────────────────────────────────────────────────────
	SaleCreated          string
	SaleNotFound         string
	InvalidSaleItems     string
	SaleMustHaveItems    string
	InvalidQuantity      string
	InvalidPaymentAmount string
	SaleAlreadySynced    string

	// ── Customers ────────────────────────────────────────────────────────────
	CustomerNotFound        string
	CustomerCreated         string
	CustomerUpdated         string
	CustomerDeleted         string
	PaymentRecorded         string
	CustomerNameRequired    string
	InsufficientDebtPayment string

	// ── Staff ────────────────────────────────────────────────────────────────
	StaffNotFound          string
	StaffCreated           string
	StaffDeactivated       string
	StaffAlreadyExists     string
	StaffNameRequired      string
	CannotDeactivateOwner  string
	InvalidStaffCredentials string

	// ── Shop ─────────────────────────────────────────────────────────────────
	ShopNotFound    string
	ShopCreated     string
	ShopUpdated     string
	ShopNameRequired string

	// ── Sync ─────────────────────────────────────────────────────────────────
	SyncCompleted string
	SyncFailed    string
	SyncConflict  string

	// ── Notifications ─────────────────────────────────────────────────────────
	NotificationNotFound string
	PreferencesUpdated   string
	FCMTokenSaved        string
	MarkedAsRead         string
	MarkedAllAsRead      string

	// FCM push text — use fmt.Sprintf with these format strings.
	// NotifLowStockBody    args: productName (string), currentStock (int)
	// NotifLargeSaleBody   args: staffName (string), totalAmount (float64)
	// NotifDebtPaymentBody args: customerName (string), amountPaid (float64)
	// NotifStaffLoginBody  args: staffName (string)
	NotifLowStockTitle    string
	NotifLowStockBody     string
	NotifLargeSaleTitle   string
	NotifLargeSaleBody    string
	NotifDebtPaymentTitle string
	NotifDebtPaymentBody  string
	NotifStaffLoginTitle  string
	NotifStaffLoginBody   string

	// ── User / password flows ─────────────────────────────────────────────────
	ForgotPasswordSent    string
	PasswordResetSuccess  string
	VerificationCodeSent  string
}

// EN contains all English strings.
var EN = Messages{
	// Auth
	InvalidCredentials:  "Invalid email or password.",
	Unauthorized:        "You are not authorized to perform this action.",
	TokenExpired:        "Your session has expired. Please log in again.",
	AccountDeactivated:  "Your account has been deactivated. Please contact support.",
	ForbiddenRole:       "Access denied: insufficient permissions.",
	PasswordTooShort:    "Password must be at least 8 characters.",
	InvalidEmail:        "Please enter a valid email address.",
	MissingAuthToken:    "Missing authorization token.",
	InvalidTokenFormat:  "Invalid authorization token format.",
	InvalidToken:        "Invalid or expired token.",
	BearerRequired:      "Authorization scheme must be Bearer.",

	// General
	NotFound:             "The requested resource was not found.",
	InternalError:        "An internal server error occurred. Please try again.",
	ValidationError:      "Validation failed. Please check your input.",
	Success:              "Success.",
	Created:              "Created successfully.",
	Updated:              "Updated successfully.",
	Deleted:              "Deleted successfully.",
	InvalidID:            "Invalid ID format.",
	MissingRequiredField: "A required field is missing.",
	BadRequest:           "Bad request. Please check your input.",
	InvalidJSON:          "Invalid JSON body.",

	// Products
	ProductNotFound:          "Product not found.",
	ProductCreated:           "Product created successfully.",
	ProductUpdated:           "Product updated successfully.",
	ProductDeleted:           "Product deleted successfully.",
	InsufficientStock:        "Insufficient stock.",
	InvalidPrice:             "Price must be a positive number.",
	InvalidStockQuantity:     "Stock quantity must be a non-negative number.",
	ProductNameRequired:      "Product name is required.",
	LowStockThresholdInvalid: "Low stock threshold must be a non-negative number.",

	// Sales
	SaleCreated:          "Sale recorded successfully.",
	SaleNotFound:         "Sale not found.",
	InvalidSaleItems:     "Invalid sale items.",
	SaleMustHaveItems:    "A sale must contain at least one item.",
	InvalidQuantity:      "Quantity must be greater than zero.",
	InvalidPaymentAmount: "Payment amount must be greater than zero.",
	SaleAlreadySynced:    "This sale has already been synced.",

	// Customers
	CustomerNotFound:        "Customer not found.",
	CustomerCreated:         "Customer created successfully.",
	CustomerUpdated:         "Customer updated successfully.",
	CustomerDeleted:         "Customer deleted successfully.",
	PaymentRecorded:         "Payment recorded successfully.",
	CustomerNameRequired:    "Customer name is required.",
	InsufficientDebtPayment: "Payment amount cannot exceed the outstanding debt.",

	// Staff
	StaffNotFound:           "Staff member not found.",
	StaffCreated:            "Staff account created successfully.",
	StaffDeactivated:        "Staff account deactivated.",
	StaffAlreadyExists:      "A staff member with this email already exists.",
	StaffNameRequired:       "Staff name is required.",
	CannotDeactivateOwner:   "You cannot deactivate an owner account.",
	InvalidStaffCredentials: "Invalid staff credentials.",

	// Shop
	ShopNotFound:     "Shop not found.",
	ShopCreated:      "Shop created successfully.",
	ShopUpdated:      "Shop updated successfully.",
	ShopNameRequired: "Shop name is required.",

	// Sync
	SyncCompleted: "Sync completed successfully.",
	SyncFailed:    "Sync failed. Please try again.",
	SyncConflict:  "Sync conflict detected. Please resolve and retry.",

	// Notifications
	NotificationNotFound: "Notification not found.",
	PreferencesUpdated:   "Notification preferences updated.",
	FCMTokenSaved:        "FCM token saved.",
	MarkedAsRead:         "Notification marked as read.",
	MarkedAllAsRead:      "All notifications marked as read.",

	// FCM push text
	NotifLowStockTitle:    "⚠️ Low Stock",
	NotifLowStockBody:     "%s only has %d unit(s) left in stock.",
	NotifLargeSaleTitle:   "💰 Large Sale Recorded",
	NotifLargeSaleBody:    "%s recorded a sale of %.0f FCFA.",
	NotifDebtPaymentTitle: "💳 Debt Payment Received",
	NotifDebtPaymentBody:  "%s paid %.0f FCFA.",
	NotifStaffLoginTitle:  "👤 Staff Login",
	NotifStaffLoginBody:   "%s just logged in.",

	// User / password flows
	ForgotPasswordSent:   "If that email is registered, a reset code has been sent.",
	PasswordResetSuccess: "Password reset successfully. Please log in with your new password.",
	VerificationCodeSent: "Verification code sent.",
}

// FR contains all French strings (default fallback for Cameroon).
var FR = Messages{
	// Auth
	InvalidCredentials:  "Email ou mot de passe incorrect.",
	Unauthorized:        "Vous n'êtes pas autorisé à effectuer cette action.",
	TokenExpired:        "Votre session a expiré. Veuillez vous reconnecter.",
	AccountDeactivated:  "Votre compte a été désactivé. Veuillez contacter le support.",
	ForbiddenRole:       "Accès refusé : permissions insuffisantes.",
	PasswordTooShort:    "Le mot de passe doit comporter au moins 8 caractères.",
	InvalidEmail:        "Veuillez entrer une adresse e-mail valide.",
	MissingAuthToken:    "Jeton d'autorisation manquant.",
	InvalidTokenFormat:  "Format du jeton d'autorisation invalide.",
	InvalidToken:        "Jeton invalide ou expiré.",
	BearerRequired:      "Le schéma d'autorisation doit être Bearer.",

	// General
	NotFound:             "La ressource demandée est introuvable.",
	InternalError:        "Une erreur interne est survenue. Veuillez réessayer.",
	ValidationError:      "Validation échouée. Vérifiez vos informations.",
	Success:              "Succès.",
	Created:              "Créé avec succès.",
	Updated:              "Mis à jour avec succès.",
	Deleted:              "Supprimé avec succès.",
	InvalidID:            "Format d'identifiant invalide.",
	MissingRequiredField: "Un champ obligatoire est manquant.",
	BadRequest:           "Requête incorrecte. Vérifiez vos informations.",
	InvalidJSON:          "Corps JSON invalide.",

	// Products
	ProductNotFound:          "Produit introuvable.",
	ProductCreated:           "Produit créé avec succès.",
	ProductUpdated:           "Produit mis à jour avec succès.",
	ProductDeleted:           "Produit supprimé avec succès.",
	InsufficientStock:        "Stock insuffisant.",
	InvalidPrice:             "Le prix doit être un nombre positif.",
	InvalidStockQuantity:     "La quantité en stock doit être un nombre positif ou nul.",
	ProductNameRequired:      "Le nom du produit est obligatoire.",
	LowStockThresholdInvalid: "Le seuil de stock faible doit être un nombre positif ou nul.",

	// Sales
	SaleCreated:          "Vente enregistrée avec succès.",
	SaleNotFound:         "Vente introuvable.",
	InvalidSaleItems:     "Articles de vente invalides.",
	SaleMustHaveItems:    "Une vente doit contenir au moins un article.",
	InvalidQuantity:      "La quantité doit être supérieure à zéro.",
	InvalidPaymentAmount: "Le montant du paiement doit être supérieur à zéro.",
	SaleAlreadySynced:    "Cette vente a déjà été synchronisée.",

	// Customers
	CustomerNotFound:        "Client introuvable.",
	CustomerCreated:         "Client créé avec succès.",
	CustomerUpdated:         "Client mis à jour avec succès.",
	CustomerDeleted:         "Client supprimé avec succès.",
	PaymentRecorded:         "Paiement enregistré avec succès.",
	CustomerNameRequired:    "Le nom du client est obligatoire.",
	InsufficientDebtPayment: "Le montant du paiement ne peut pas dépasser la dette en cours.",

	// Staff
	StaffNotFound:           "Membre du personnel introuvable.",
	StaffCreated:            "Compte du personnel créé avec succès.",
	StaffDeactivated:        "Compte du personnel désactivé.",
	StaffAlreadyExists:      "Un membre du personnel avec cet e-mail existe déjà.",
	StaffNameRequired:       "Le nom du personnel est obligatoire.",
	CannotDeactivateOwner:   "Vous ne pouvez pas désactiver un compte propriétaire.",
	InvalidStaffCredentials: "Identifiants du personnel invalides.",

	// Shop
	ShopNotFound:     "Boutique introuvable.",
	ShopCreated:      "Boutique créée avec succès.",
	ShopUpdated:      "Boutique mise à jour avec succès.",
	ShopNameRequired: "Le nom de la boutique est obligatoire.",

	// Sync
	SyncCompleted: "Synchronisation terminée avec succès.",
	SyncFailed:    "La synchronisation a échoué. Veuillez réessayer.",
	SyncConflict:  "Conflit de synchronisation détecté. Résolvez et réessayez.",

	// Notifications
	NotificationNotFound: "Notification introuvable.",
	PreferencesUpdated:   "Préférences de notification mises à jour.",
	FCMTokenSaved:        "Token FCM enregistré.",
	MarkedAsRead:         "Notification marquée comme lue.",
	MarkedAllAsRead:      "Toutes les notifications ont été marquées comme lues.",

	// FCM push text
	NotifLowStockTitle:    "⚠️ Stock faible",
	NotifLowStockBody:     "%s n'a plus que %d unité(s) en stock.",
	NotifLargeSaleTitle:   "💰 Grande vente enregistrée",
	NotifLargeSaleBody:    "%s a enregistré une vente de %.0f FCFA.",
	NotifDebtPaymentTitle: "💳 Paiement de dette reçu",
	NotifDebtPaymentBody:  "%s a payé %.0f FCFA.",
	NotifStaffLoginTitle:  "👤 Connexion du personnel",
	NotifStaffLoginBody:   "%s vient de se connecter.",

	// User / password flows
	ForgotPasswordSent:   "Si cet e-mail est enregistré, un code de réinitialisation a été envoyé.",
	PasswordResetSuccess: "Mot de passe réinitialisé avec succès. Connectez-vous avec votre nouveau mot de passe.",
	VerificationCodeSent: "Code de vérification envoyé.",
}

// Get returns the Messages for the given locale. Defaults to FR.
func Get(locale string) Messages {
	if locale == "en" {
		return EN
	}
	return FR
}
