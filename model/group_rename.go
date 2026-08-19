package model

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

type GroupRename struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type GroupSettingsRequest struct {
	Options map[string]string `json:"options"`
	Renames []GroupRename     `json:"renames"`
}

type GroupRenameResult struct {
	Users             int `json:"users"`
	Tokens            int `json:"tokens"`
	Channels          int `json:"channels"`
	SubscriptionPlans int `json:"subscription_plans"`
	Subscriptions     int `json:"subscriptions"`
}

func renameGroupKey[V any](groups map[string]V, renamed map[string]string) (map[string]V, error) {
	updated := make(map[string]V, len(groups))
	for group, value := range groups {
		if replacement, ok := renamed[group]; ok {
			group = replacement
		}
		if _, exists := updated[group]; exists {
			return nil, fmt.Errorf("group setting collision: %s", group)
		}
		updated[group] = value
	}
	return updated, nil
}

func marshalRenamedGroupOption[T any](value string, renamed map[string]string, transform func(T) (T, error)) (string, error) {
	var parsed T
	if err := common.UnmarshalJsonStr(value, &parsed); err != nil {
		return "", err
	}
	updated, err := transform(parsed)
	if err != nil {
		return "", err
	}
	bytes, err := common.Marshal(updated)
	return string(bytes), err
}

func validGroupName(name string) bool {
	return name != "" && name == strings.TrimSpace(name) && utf8.RuneCountInString(name) <= 64 && name != "auto" && !strings.Contains(name, ",") && !strings.HasPrefix(name, "+:") && !strings.HasPrefix(name, "-:") && !strings.ContainsFunc(name, unicode.IsControl)
}

func UpdateGroupSettings(request GroupSettingsRequest) (GroupRenameResult, error) {
	var result GroupRenameResult
	if len(request.Options) == 0 {
		return result, fmt.Errorf("group settings are required")
	}

	renamed := make(map[string]string, len(request.Renames))
	seenTargets := make(map[string]struct{}, len(request.Renames))
	for _, rename := range request.Renames {
		from, to := strings.TrimSpace(rename.From), strings.TrimSpace(rename.To)
		if !validGroupName(from) || !validGroupName(to) || from == to {
			return result, fmt.Errorf("invalid group rename")
		}
		if _, ok := renamed[from]; ok {
			return result, fmt.Errorf("duplicate source group: %s", from)
		}
		if _, ok := seenTargets[to]; ok {
			return result, fmt.Errorf("duplicate target group: %s", to)
		}
		seenTargets[to] = struct{}{}
		renamed[from] = to
	}
	for from, to := range renamed {
		if _, chained := renamed[to]; chained {
			return result, fmt.Errorf("chained or exchanged group rename: %s", from)
		}
	}

	var finalOptions map[string]string
	var affectedUsers []int
	var affectedPlanIDs []int
	err := DB.Transaction(func(tx *gorm.DB) error {
		var options []Option
		if err := lockForUpdate(tx).Find(&options).Error; err != nil {
			return err
		}
		currentOptions := make(map[string]string, len(options))
		for _, option := range options {
			currentOptions[option.Key] = option.Value
		}
		finalOptions = make(map[string]string, len(request.Options)+6)
		for key, value := range request.Options {
			finalOptions[key] = value
		}
		for _, key := range []string{"GroupRatio", "TopupGroupRatio", "UserUsableGroups", "GroupGroupRatio", "AutoGroups", "ModelRequestRateLimitGroup", "group_ratio_setting.group_special_usable_group"} {
			if _, submitted := finalOptions[key]; !submitted {
				if value, exists := currentOptions[key]; exists {
					finalOptions[key] = value
				}
			}
		}

		if len(renamed) > 0 {
			for _, key := range []string{"GroupRatio", "TopupGroupRatio"} {
				if value, exists := finalOptions[key]; exists {
					updated, err := marshalRenamedGroupOption(value, renamed, func(groups map[string]float64) (map[string]float64, error) { return renameGroupKey(groups, renamed) })
					if err != nil {
						return fmt.Errorf("invalid %s: %w", key, err)
					}
					finalOptions[key] = updated
				}
			}
			if value, exists := finalOptions["UserUsableGroups"]; exists {
				updated, err := marshalRenamedGroupOption(value, renamed, func(groups map[string]string) (map[string]string, error) { return renameGroupKey(groups, renamed) })
				if err != nil {
					return fmt.Errorf("invalid UserUsableGroups: %w", err)
				}
				finalOptions["UserUsableGroups"] = updated
			}
			if value, exists := finalOptions["ModelRequestRateLimitGroup"]; exists {
				updated, err := marshalRenamedGroupOption(value, renamed, func(groups map[string][2]int) (map[string][2]int, error) { return renameGroupKey(groups, renamed) })
				if err != nil {
					return fmt.Errorf("invalid ModelRequestRateLimitGroup: %w", err)
				}
				finalOptions["ModelRequestRateLimitGroup"] = updated
			}
			if value, exists := finalOptions["GroupGroupRatio"]; exists {
				updated, err := marshalRenamedGroupOption(value, renamed, func(groups map[string]map[string]float64) (map[string]map[string]float64, error) {
					updated, err := renameGroupKey(groups, renamed)
					if err != nil {
						return nil, err
					}
					for group, ratios := range updated {
						updated[group], err = renameGroupKey(ratios, renamed)
						if err != nil {
							return nil, err
						}
					}
					return updated, nil
				})
				if err != nil {
					return fmt.Errorf("invalid GroupGroupRatio: %w", err)
				}
				finalOptions["GroupGroupRatio"] = updated
			}
			if value, exists := finalOptions["AutoGroups"]; exists {
				updated, err := marshalRenamedGroupOption(value, renamed, func(groups []string) ([]string, error) {
					for i, group := range groups {
						if replacement, ok := renamed[group]; ok {
							groups[i] = replacement
						}
					}
					return groups, nil
				})
				if err != nil {
					return fmt.Errorf("invalid AutoGroups: %w", err)
				}
				finalOptions["AutoGroups"] = updated
			}
			if value, exists := finalOptions["group_ratio_setting.group_special_usable_group"]; exists {
				updated, err := marshalRenamedGroupOption(value, renamed, func(groups map[string]map[string]string) (map[string]map[string]string, error) {
					updated, err := renameGroupKey(groups, renamed)
					if err != nil {
						return nil, err
					}
					for group, settings := range updated {
						rewritten := make(map[string]string, len(settings))
						for name, description := range settings {
							prefix, base := "", name
							if strings.HasPrefix(name, "+:") || strings.HasPrefix(name, "-:") {
								prefix, base = name[:2], name[2:]
							}
							if replacement, ok := renamed[base]; ok {
								name = prefix + replacement
							}
							if _, exists := rewritten[name]; exists {
								return nil, fmt.Errorf("group setting collision: %s", name)
							}
							rewritten[name] = description
						}
						updated[group] = rewritten
					}
					return updated, nil
				})
				if err != nil {
					return fmt.Errorf("invalid group_special_usable_group: %w", err)
				}
				finalOptions["group_ratio_setting.group_special_usable_group"] = updated
			}
		}

		var currentGroups map[string]float64
		if err := common.UnmarshalJsonStr(currentOptions["GroupRatio"], &currentGroups); err != nil {
			return fmt.Errorf("invalid stored group ratio: %w", err)
		}
		var submittedGroups map[string]float64
		if err := common.UnmarshalJsonStr(finalOptions["GroupRatio"], &submittedGroups); err != nil {
			return fmt.Errorf("invalid submitted group ratio: %w", err)
		}
		for group := range submittedGroups {
			if !validGroupName(group) {
				return fmt.Errorf("invalid group name: %s", group)
			}
		}
		for group := range currentGroups {
			if _, remains := submittedGroups[group]; !remains {
				if _, renamed := renamed[group]; !renamed {
					return fmt.Errorf("group deletion requires explicit rename: %s", group)
				}
			}
		}
		for from, to := range renamed {
			if _, exists := currentGroups[from]; !exists {
				return fmt.Errorf("source group does not exist: %s", from)
			}
			if _, exists := currentGroups[to]; exists {
				return fmt.Errorf("target group already exists: %s", to)
			}
			if _, exists := submittedGroups[from]; exists {
				return fmt.Errorf("renamed source remains in group settings: %s", from)
			}
			if _, exists := submittedGroups[to]; !exists {
				return fmt.Errorf("renamed target missing from group settings: %s", to)
			}
		}
		for key, target := range map[string]any{
			"TopupGroupRatio":            &map[string]float64{},
			"UserUsableGroups":           &map[string]string{},
			"GroupGroupRatio":            &map[string]map[string]float64{},
			"AutoGroups":                 &[]string{},
			"ModelRequestRateLimitGroup": &map[string][2]int{},
			"group_ratio_setting.group_special_usable_group": &map[string]map[string]string{},
		} {
			if value, exists := finalOptions[key]; exists {
				if err := common.UnmarshalJsonStr(value, target); err != nil {
					return fmt.Errorf("invalid %s: %w", key, err)
				}
			}
		}
		if value, exists := finalOptions["AutoGroupEnabled"]; exists && value != "true" && value != "false" {
			return fmt.Errorf("invalid AutoGroupEnabled")
		}
		if value, exists := finalOptions["DefaultUseAutoGroup"]; exists && value != "true" && value != "false" {
			return fmt.Errorf("invalid DefaultUseAutoGroup")
		}
		for key, value := range finalOptions {
			option := Option{Key: key}
			if err := tx.FirstOrCreate(&option, Option{Key: key}).Error; err != nil {
				return err
			}
			option.Value = value
			if err := tx.Save(&option).Error; err != nil {
				return err
			}
		}

		for from, to := range renamed {
			var users []User
			if err := lockForUpdate(tx).Select("id").Where(commonGroupCol+" = ?", from).Find(&users).Error; err != nil {
				return err
			}
			if len(users) > 0 {
				if err := tx.Model(&User{}).Where(commonGroupCol+" = ?", from).Update("group", to).Error; err != nil {
					return err
				}
				result.Users += len(users)
				for _, user := range users {
					affectedUsers = append(affectedUsers, user.Id)
				}
			}
			var tokens []Token
			if err := lockForUpdate(tx).Select("id", "user_id").Where(commonGroupCol+" = ?", from).Find(&tokens).Error; err != nil {
				return err
			}
			if len(tokens) > 0 {
				if err := tx.Model(&Token{}).Where(commonGroupCol+" = ?", from).Update("group", to).Error; err != nil {
					return err
				}
				result.Tokens += len(tokens)
				for _, token := range tokens {
					affectedUsers = append(affectedUsers, token.UserId)
				}
			}
			var plans []SubscriptionPlan
			if err := lockForUpdate(tx).Select("id").Where("upgrade_group = ? OR downgrade_group = ?", from, from).Find(&plans).Error; err != nil {
				return err
			}
			if len(plans) > 0 {
				if err := tx.Model(&SubscriptionPlan{}).Where("upgrade_group = ?", from).Update("upgrade_group", to).Error; err != nil {
					return err
				}
				if err := tx.Model(&SubscriptionPlan{}).Where("downgrade_group = ?", from).Update("downgrade_group", to).Error; err != nil {
					return err
				}
				result.SubscriptionPlans += len(plans)
				for _, plan := range plans {
					affectedPlanIDs = append(affectedPlanIDs, plan.Id)
				}
			}
			var subscriptions []UserSubscription
			if err := lockForUpdate(tx).Select("id").Where("upgrade_group = ? OR prev_user_group = ? OR downgrade_group = ?", from, from, from).Find(&subscriptions).Error; err != nil {
				return err
			}
			if len(subscriptions) > 0 {
				for _, column := range []string{"upgrade_group", "prev_user_group", "downgrade_group"} {
					if err := tx.Model(&UserSubscription{}).Where(column+" = ?", from).Update(column, to).Error; err != nil {
						return err
					}
				}
				result.Subscriptions += len(subscriptions)
			}
		}
		if len(renamed) > 0 {
			var channels []Channel
			if err := lockForUpdate(tx).Find(&channels).Error; err != nil {
				return err
			}
			for _, channel := range channels {
				groups, changed := channel.GetGroups(), false
				seen := make(map[string]struct{}, len(groups))
				for i, group := range groups {
					if replacement, ok := renamed[group]; ok {
						groups[i], changed = replacement, true
					}
					if _, duplicate := seen[groups[i]]; duplicate {
						return fmt.Errorf("channel group collision: channel_id=%d group=%s", channel.Id, groups[i])
					}
					seen[groups[i]] = struct{}{}
				}
				if !changed {
					continue
				}
				channel.Group = strings.Join(groups, ",")
				if err := tx.Model(&Channel{}).Where("id = ?", channel.Id).Update("group", channel.Group).Error; err != nil {
					return err
				}
				if err := channel.UpdateAbilities(tx); err != nil {
					return err
				}
				result.Channels++
			}
		}
		return nil
	})
	if err != nil {
		return result, err
	}
	for key, value := range finalOptions {
		if err := updateOptionMap(key, value); err != nil {
			return result, err
		}
	}
	seenUsers := make(map[int]struct{})
	for _, userID := range affectedUsers {
		if _, ok := seenUsers[userID]; ok {
			continue
		}
		seenUsers[userID] = struct{}{}
		if err := RefreshUserGroupCache(userID); err != nil {
			common.SysError(fmt.Sprintf("group rename refresh user cache user_id=%d: %v", userID, err))
		}
		if err := InvalidateUserTokensCache(userID); err != nil {
			common.SysError(fmt.Sprintf("group rename invalidate token cache user_id=%d: %v", userID, err))
		}
	}
	for _, planID := range affectedPlanIDs {
		InvalidateSubscriptionPlanCache(planID)
	}
	InitChannelCache()
	return result, nil
}
